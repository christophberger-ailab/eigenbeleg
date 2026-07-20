// Command eigenbeleg generates an Eigenbeleg (self-made receipt) PDF.
//
// All settings can be given as command line flags, environment variables
// (EIGENBELEG_<FLAGNAME>), or entries in a config file (see the start
// package for details).
package main

import (
	"bufio"
	"fmt"
	"io"
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
	subject   = flag.StringP("subject", "s", "", "subject of the transaction (default: Privatentnahme or Privateinlage)")
	recipient = flag.StringP("recipient", "r", "", "recipient of the payment (required)")
	typ       = flag.StringP("type", "t", "", "type of expenditure (default: Privatentnahme or Privateinlage)")
	reason    = flag.StringP("reason", "g", "", "reason for the Eigenbeleg (required)")
	date      = flag.StringP("date", "d", "", "date as YYYY-MM-DD (default: today)")
	signature = flag.StringP("signature", "i", "", "path to signature image, PNG or JPEG (required)")
	header    = flag.StringP("header", "H", "", "path to header image, PNG or JPEG")
	output    = flag.StringP("output", "o", "", "path of the PDF file to write (default: ./<date> Eigenbeleg <number>.pdf)")
)

const description = "Generate an Eigenbeleg (self-made receipt) PDF."

func main() {
	start.SetDescription(description)
	flag.Usage = func() { printUsage(os.Stdout) }
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		flag.Usage()
		return nil
	}

	if err := start.Parse(); err != nil {
		return err
	}

	var missing []string
	for _, f := range []struct{ name, value string }{
		{"number", *number},
		{"amount", *amount},
		{"recipient", *recipient},
		{"reason", *reason},
		{"signature", *signature},
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

	receipt := eigenbeleg.Receipt{
		Number:        *number,
		Amount:        amountVal,
		Subject:       *subject,
		Recipient:     *recipient,
		Type:          *typ,
		Reason:        *reason,
		Date:          dateVal,
		SignaturePath: *signature,
		HeaderPath:    *header,
	}
	if err := receipt.Validate(); err != nil {
		return err
	}

	outPath := *output
	if outPath == "" {
		outPath = defaultOutputPath(receipt.Date, receipt.Number)
	}

	printFields(os.Stdout, receipt, outPath)
	ok, err := confirm(os.Stdin, os.Stdout)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("Aborted.")
		return nil
	}

	pdfBytes, err := eigenbeleg.Generate(receipt)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outPath, pdfBytes, 0o644); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}
	return nil
}

// printUsage prints the tool description and the flag list. It backs
// flag.Usage, so pflag invokes it for --help, and run() invokes it when the
// tool is called without any arguments.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, description)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Usage of %s:\n", os.Args[0])
	fmt.Fprint(w, flag.CommandLine.FlagUsages())
}

// printFields lists the receipt fields with inputs and defaults filled in,
// plus the effective output path.
func printFields(w io.Writer, r eigenbeleg.Receipt, outPath string) {
	kind := "Einnahme"
	if r.Amount < 0 {
		kind = "Ausgabe"
	}
	headerPath := r.HeaderPath
	if headerPath == "" {
		headerPath = "(none)"
	}
	for _, f := range []struct{ name, value string }{
		{"number", r.Number},
		{"amount", eigenbeleg.FormatAmount(r.Amount) + " (" + kind + ")"},
		{"subject", r.Subject},
		{"recipient", r.Recipient},
		{"type", r.Type},
		{"reason", r.Reason},
		{"date", r.Date.Format("2006-01-02")},
		{"signature", r.SignaturePath},
		{"header", headerPath},
		{"output", outPath},
	} {
		fmt.Fprintf(w, "%-10s %s\n", f.name+":", f.value)
	}
}

// confirm prompts on out and reads a single line from in. An empty line
// (just Enter, or end of input) or a "y"/"yes" answer confirms.
func confirm(in io.Reader, out io.Writer) (bool, error) {
	fmt.Fprint(out, "Generate PDF? [Y/n] ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("reading confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		return true, nil
	}
	return false, nil
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

// parseDate parses YYYY-MM-DD (ISO format); an empty string yields the zero
// time, which makes the library default to the current date.
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: expected format YYYY-MM-DD", s)
	}
	return t, nil
}

// defaultOutputPath builds "./<date> Eigenbeleg <number>.pdf" with the date
// as YYYY-MM-DD, falling back to today when no date was given.
func defaultOutputPath(date time.Time, number string) string {
	if date.IsZero() {
		date = time.Now()
	}
	return fmt.Sprintf("%s Eigenbeleg %s.pdf", date.Format("2006-01-02"), number)
}
