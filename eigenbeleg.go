// Package eigenbeleg generates an "Eigenbeleg" (self-made receipt) as a PDF
// document, for use in German bookkeeping when no original receipt exists.
package eigenbeleg

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
)

// Receipt holds the input data for a single Eigenbeleg.
type Receipt struct {
	// Number is the receipt number ("Nr.").
	Number string
	// Amount is the receipt amount. A positive amount is a revenue
	// ("Einnahme"), a negative amount an expense ("Ausgabe"). The PDF
	// always shows the absolute value.
	Amount float64
	// Subject describes the transaction ("Vorgang"). If empty, it
	// defaults to "Privatentnahme" for negative amounts and
	// "Privateinlage" for positive amounts.
	Subject string
	// Recipient is the receiver of the payment ("Empfänger").
	Recipient string
	// Type is the type of expenditure ("Art der Aufwendung"). If empty,
	// it defaults to "Privatentnahme" for negative amounts and
	// "Privateinlage" for positive amounts.
	Type string
	// Reason explains why no original receipt exists ("Grund des
	// Eigenbelegs").
	Reason string
	// Date is the receipt date. The zero value defaults to the current
	// date.
	Date time.Time
	// SignaturePath is the path to a PNG or JPEG image of the signature.
	// It is mandatory.
	SignaturePath string
	// HeaderPath is the path to an optional PNG or JPEG header image.
	HeaderPath string
}

// Page geometry in millimeters (A4 portrait).
const (
	pageWidth    = 210.0
	marginLeft   = 25.0
	marginTop    = 20.0
	marginRight  = 25.0
	usableWidth  = pageWidth - marginLeft - marginRight
	labelWidth   = 60.0
	rowHeight    = 8.0
	headerMaxH   = 30.0
	sigMaxWidth  = 60.0
	sigMaxHeight = 20.0
)

// Validate checks the mandatory fields and applies the defaults for
// Subject, Type, and Date, modifying r in place. Generate validates on its
// own; calling Validate beforehand lets callers inspect the effective field
// values, e.g. to show them to the user.
func (r *Receipt) Validate() error {
	return validate(r)
}

// Generate renders the receipt as an A4 PDF and returns the PDF bytes.
func Generate(r Receipt) ([]byte, error) {
	if err := validate(&r); err != nil {
		return nil, err
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(marginLeft, marginTop, marginRight)
	pdf.AddPage()

	if r.HeaderPath != "" {
		placeImage(pdf, r.HeaderPath, marginLeft, pdf.GetY(), usableWidth, headerMaxH)
		pdf.Ln(4)
	}

	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(0, 14, "Eigenbeleg", "", 1, "L", false, 0, "")
	pdf.Ln(4)

	amountLabel := "Einnahme"
	if r.Amount < 0 {
		amountLabel = "Ausgabe"
	}
	fields := []struct{ label, value string }{
		{"Nr.", r.Number},
		{amountLabel, FormatAmount(r.Amount)},
		{"Vorgang", r.Subject},
		{"Empfänger", r.Recipient},
		{"Art der Aufwendung", r.Type},
		{"Grund des Eigenbelegs", r.Reason},
	}
	for _, f := range fields {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(labelWidth, rowHeight, tr(f.label), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.MultiCell(usableWidth-labelWidth, rowHeight, tr(f.value), "", "L", false)
	}

	// Date and signature image, bottom-aligned on a common line.
	sigX := marginLeft + 90.0
	lineY := pdf.GetY() + 12 + sigMaxHeight
	sigW, sigH := placeImage(pdf, r.SignaturePath, sigX, -1, sigMaxWidth, sigMaxHeight)
	pdf.ImageOptions(r.SignaturePath, sigX, lineY-sigH, sigW, sigH, false,
		fpdf.ImageOptions{ReadDpi: true}, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.SetXY(marginLeft, lineY-rowHeight)
	pdf.CellFormat(labelWidth, rowHeight, r.Date.Format("02.01.2006"), "", 0, "L", false, 0, "")

	// Rule below date and signature, with their captions underneath.
	ruleY := lineY + 1
	pdf.Line(marginLeft, ruleY, pageWidth-marginRight, ruleY)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(marginLeft, ruleY+1)
	pdf.CellFormat(labelWidth, 5, "Datum", "", 0, "L", false, 0, "")
	pdf.SetXY(sigX, ruleY+1)
	pdf.CellFormat(sigMaxWidth, 5, "Unterschrift", "", 0, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("generating PDF: %w", err)
	}
	return buf.Bytes(), nil
}

// validate checks mandatory fields and applies the defaults for Subject,
// Type, and Date. It modifies r in place.
func validate(r *Receipt) error {
	if r.SignaturePath == "" {
		return fmt.Errorf("signature image is mandatory")
	}
	for _, img := range []struct{ name, path string }{
		{"signature", r.SignaturePath},
		{"header", r.HeaderPath},
	} {
		if img.path == "" {
			continue
		}
		switch strings.ToLower(filepath.Ext(img.path)) {
		case ".png", ".jpg", ".jpeg":
		default:
			return fmt.Errorf("%s image %q: only PNG and JPEG images are supported", img.name, img.path)
		}
		if _, err := os.Stat(img.path); err != nil {
			return fmt.Errorf("%s image: %w", img.name, err)
		}
	}
	if r.Subject == "" {
		if r.Amount < 0 {
			r.Subject = "Privatentnahme"
		} else {
			r.Subject = "Privateinlage"
		}
	}
	if r.Type == "" {
		if r.Amount < 0 {
			r.Type = "Privatentnahme"
		} else {
			r.Type = "Privateinlage"
		}
	}
	if r.Date.IsZero() {
		r.Date = time.Now()
	}
	return nil
}

// placeImage scales the image at path to fit within maxW x maxH, keeping its
// aspect ratio, and returns the scaled size. If y is negative, the image is
// only registered and measured, not drawn.
func placeImage(pdf *fpdf.Fpdf, path string, x, y, maxW, maxH float64) (w, h float64) {
	opts := fpdf.ImageOptions{ReadDpi: true}
	info := pdf.RegisterImageOptions(path, opts)
	if info == nil {
		return 0, 0
	}
	w, h = info.Extent()
	if scale := math.Min(maxW/w, maxH/h); scale < 1 {
		w, h = w*scale, h*scale
	}
	if y >= 0 {
		pdf.ImageOptions(path, x, y, w, h, false, opts, 0, "")
		pdf.SetY(y + h)
	}
	return w, h
}

// FormatAmount formats the absolute value of amount in German notation with
// a trailing Euro sign, e.g. 1.234,56 €.
func FormatAmount(amount float64) string {
	s := strconv.FormatFloat(math.Abs(amount), 'f', 2, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, d := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(d)
	}
	return b.String() + "," + frac + " €"
}
