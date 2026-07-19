package eigenbeleg

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
		want   string
	}{
		{"zero", 0, "0,00 €"},
		{"typical", 1234.56, "1.234,56 €"},
		{"negative becomes positive", -1234.56, "1.234,56 €"},
		{"millions", 1000000, "1.000.000,00 €"},
		{"half euro rounding", 12.5, "12,50 €"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAmount(tt.amount)
			if got != tt.want {
				t.Errorf("FormatAmount(%v) = %q, want %q", tt.amount, got, tt.want)
			}
		})
	}
}

// writePNG writes a tiny valid PNG image to path.
func writePNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding png to %s: %v", path, err)
	}
}

func baseReceipt(sigPath string) Receipt {
	return Receipt{
		Number:        "2026-001",
		Amount:        -42.50,
		Subject:       "Parkgebühren",
		Recipient:     "Parkhaus Musterstadt",
		Reason:        "Kein Beleg erhalten",
		SignaturePath: sigPath,
	}
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	sigPath := filepath.Join(dir, "signature.png")
	writePNG(t, sigPath)

	r := baseReceipt(sigPath)
	pdfBytes, err := Generate(r)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("Generate() returned empty PDF bytes")
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Errorf("Generate() output does not start with %%PDF, got %q", pdfBytes[:minInt(len(pdfBytes), 20)])
	}
}

func TestGenerateWithHeaderImage(t *testing.T) {
	dir := t.TempDir()
	sigPath := filepath.Join(dir, "signature.png")
	headerPath := filepath.Join(dir, "header.png")
	writePNG(t, sigPath)
	writePNG(t, headerPath)

	r := baseReceipt(sigPath)
	r.HeaderPath = headerPath
	pdfBytes, err := Generate(r)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("Generate() returned empty PDF bytes")
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Errorf("Generate() output does not start with %%PDF, got %q", pdfBytes[:minInt(len(pdfBytes), 20)])
	}
}

func TestGenerateErrors(t *testing.T) {
	dir := t.TempDir()
	sigPath := filepath.Join(dir, "signature.png")
	writePNG(t, sigPath)

	t.Run("empty signature path", func(t *testing.T) {
		r := baseReceipt("")
		_, err := Generate(r)
		if err == nil {
			t.Fatal("expected error for empty SignaturePath, got nil")
		}
	})

	t.Run("non-existent signature file", func(t *testing.T) {
		r := baseReceipt(filepath.Join(dir, "does-not-exist.png"))
		_, err := Generate(r)
		if err == nil {
			t.Fatal("expected error for non-existent signature file, got nil")
		}
	})

	t.Run("unsupported extension", func(t *testing.T) {
		gifPath := filepath.Join(dir, "signature.gif")
		if err := os.WriteFile(gifPath, []byte("GIF89a"), 0o644); err != nil {
			t.Fatalf("writing gif file: %v", err)
		}
		r := baseReceipt(gifPath)
		_, err := Generate(r)
		if err == nil {
			t.Fatal("expected error for unsupported .gif extension, got nil")
		}
	})
}

func TestValidateTypeDefaulting(t *testing.T) {
	dir := t.TempDir()
	sigPath := filepath.Join(dir, "signature.png")
	writePNG(t, sigPath)

	t.Run("negative amount defaults to Privatentnahme", func(t *testing.T) {
		r := Receipt{Amount: -10, SignaturePath: sigPath}
		if err := validate(&r); err != nil {
			t.Fatalf("validate() error = %v", err)
		}
		if r.Type != "Privatentnahme" {
			t.Errorf("Type = %q, want %q", r.Type, "Privatentnahme")
		}
	})

	t.Run("positive amount defaults to Privateinlage", func(t *testing.T) {
		r := Receipt{Amount: 10, SignaturePath: sigPath}
		if err := validate(&r); err != nil {
			t.Fatalf("validate() error = %v", err)
		}
		if r.Type != "Privateinlage" {
			t.Errorf("Type = %q, want %q", r.Type, "Privateinlage")
		}
	})

	t.Run("negative amount defaults subject to Privatentnahme", func(t *testing.T) {
		r := Receipt{Amount: -10, SignaturePath: sigPath}
		if err := validate(&r); err != nil {
			t.Fatalf("validate() error = %v", err)
		}
		if r.Subject != "Privatentnahme" {
			t.Errorf("Subject = %q, want %q", r.Subject, "Privatentnahme")
		}
	})

	t.Run("positive amount defaults subject to Privateinlage", func(t *testing.T) {
		r := Receipt{Amount: 10, SignaturePath: sigPath}
		if err := validate(&r); err != nil {
			t.Fatalf("validate() error = %v", err)
		}
		if r.Subject != "Privateinlage" {
			t.Errorf("Subject = %q, want %q", r.Subject, "Privateinlage")
		}
	})

	t.Run("explicit subject is preserved", func(t *testing.T) {
		r := Receipt{Amount: -10, Subject: "Parkgebühren", SignaturePath: sigPath}
		if err := validate(&r); err != nil {
			t.Fatalf("validate() error = %v", err)
		}
		if r.Subject != "Parkgebühren" {
			t.Errorf("Subject = %q, want %q", r.Subject, "Parkgebühren")
		}
	})

	t.Run("explicit type is preserved", func(t *testing.T) {
		r := Receipt{Amount: -10, Type: "Sonderfall", SignaturePath: sigPath}
		if err := validate(&r); err != nil {
			t.Fatalf("validate() error = %v", err)
		}
		if r.Type != "Sonderfall" {
			t.Errorf("Type = %q, want %q", r.Type, "Sonderfall")
		}
	})

	t.Run("zero date defaults to non-zero", func(t *testing.T) {
		r := Receipt{Amount: 10, SignaturePath: sigPath}
		if err := validate(&r); err != nil {
			t.Fatalf("validate() error = %v", err)
		}
		if r.Date.IsZero() {
			t.Error("Date is zero, want it to default to the current date")
		}
	})

	t.Run("explicit date is preserved", func(t *testing.T) {
		want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
		r := Receipt{Amount: 10, SignaturePath: sigPath, Date: want}
		if err := validate(&r); err != nil {
			t.Fatalf("validate() error = %v", err)
		}
		if !r.Date.Equal(want) {
			t.Errorf("Date = %v, want %v", r.Date, want)
		}
	})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
