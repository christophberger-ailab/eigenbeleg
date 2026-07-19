// Command eigenbeleg generates an Eigenbeleg (self-made receipt) PDF.
//
// All settings can be given as command line flags, environment variables
// (EIGENBELEG_<FLAGNAME>), or entries in a config file (see the start
// package for details).
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/christophberger/eigenbeleg"
	"github.com/christophberger/start"
	flag "github.com/spf13/pflag"
)

var (
	number    = flag.StringP("number", "n", "", "receipt number (required)")
	amount    = flag.StringP("amount", "a", "", "amount, e.g. 1.234,56 or -42.50; negative for expenses (required)")
	subject   = flag.StringP("subject", "s", "", "subject of the transaction (required)")
	recipient = flag.StringP("recipient", "r", "", "recipient of the payment (required)")
	typ       = flag.StringP("type", "t", "", "type of expenditure (default: Privatentnahme or Privateinlage)")
	reason    = flag.StringP("reason", "g", "", "reason for the Eigenbeleg (required)")
	date      = flag.StringP("date", "d", "", "date as DD.MM.YYYY (default: today)")
	signature = flag.StringP("signature", "i", "", "path to signature image, PNG or JPEG (required)")
	header    = flag.StringP("header", "H", "", "path to header image, PNG or JPEG")
	output    = flag.StringP("output", "o", "", "path of the PDF file to write (required)")
)

func main() {
	start.SetDescription("Generate an Eigenbeleg (self-made receipt) PDF.")
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := start.Parse(); err != nil {
		return err
	}

	var missing []string
	for _, f := range []struct{ name, value string }{
		{"number", *number},
		{"amount", *amount},
		{"subject", *subject},
		{"recipient", *recipient},
		{"reason", *reason},
		{"signature", *signature},
		{"output", *output},
	} {
		if f.value == "" {
			missing = append(missing, "--"+f.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required input: %s", strings.Join(missing, ", "))
	}

	amountVal, err := parseAmount(*amount)
	if err != nil {
		return err
	}
	dateVal, err := parseDate(*date)
	if err != nil {
		return err
	}

	pdfBytes, err := eigenbeleg.Generate(eigenbeleg.Receipt{
		Number:        *number,
		Amount:        amountVal,
		Subject:       *subject,
		Recipient:     *recipient,
		Type:          *typ,
		Reason:        *reason,
		Date:          dateVal,
		SignaturePath: *signature,
		HeaderPath:    *header,
	})
	if err != nil {
		return err
	}

	if err := os.WriteFile(*output, pdfBytes, 0o644); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}
	return nil
}

// parseAmount accepts German notation (1.234,56) as well as plain decimal
// notation (1234.56), with an optional sign and Euro suffix.
func parseAmount(s string) (float64, error) {
	norm := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "€"))
	if strings.Contains(norm, ",") {
		norm = strings.ReplaceAll(norm, ".", "")
		norm = strings.Replace(norm, ",", ".", 1)
	}
	v, err := strconv.ParseFloat(norm, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: expected a number like 1.234,56 or -42.50", s)
	}
	return v, nil
}

// parseDate parses DD.MM.YYYY; an empty string yields the zero time, which
// makes the library default to the current date.
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("02.01.2006", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: expected format DD.MM.YYYY", s)
	}
	return t, nil
}
