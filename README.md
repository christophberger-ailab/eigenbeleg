# eigenbeleg

A Go library and CLI tool for generating an *Eigenbeleg* — a self-made
substitute receipt used in German bookkeeping when no original receipt for a
business expense or cash deposit is available.

## Library usage

```go
package main

import (
	"os"
	"time"

	"github.com/christophberger/eigenbeleg"
)

func main() {
	pdfBytes, err := eigenbeleg.Generate(eigenbeleg.Receipt{
		Number:        "2026-001",
		Amount:        -42.50, // negative = expense ("Ausgabe")
		Subject:       "Parkgebühren",
		Recipient:     "Parkhaus Musterstadt",
		Reason:        "Kein Beleg erhalten",
		Date:          time.Now(), // zero value defaults to today
		SignaturePath: "signature.png",
		HeaderPath:    "header.png", // optional
	})
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("eigenbeleg.pdf", pdfBytes, 0o644); err != nil {
		panic(err)
	}
}
```

If `Type` is left empty, it defaults to `"Privatentnahme"` for a negative
amount and `"Privateinlage"` for a positive amount.

## CLI usage

### Install

```sh
go install github.com/christophberger/eigenbeleg/cmd/eigenbeleg@latest
```

### Flags

All settings can be supplied as command line flags, as environment variables
named `EIGENBELEG_<FLAGNAME>` (uppercase, underscores for dashes), or as
entries in a config file — see the
[`start`](https://github.com/christophberger/start) package for details on
config file locations and precedence.

| Flag              | Short | Env var                | Required | Description                                                          |
| ------------------ | ----- | ----------------------- | -------- | ---------------------------------------------------------------------- |
| `--number`         | `-n`  | `EIGENBELEG_NUMBER`     | yes      | Receipt number                                                        |
| `--amount`         | `-a`  | `EIGENBELEG_AMOUNT`     | yes      | Amount, e.g. `1.234,56` or `-42.50`; negative for expenses             |
| `--subject`        | `-s`  | `EIGENBELEG_SUBJECT`    | yes      | Subject of the transaction                                             |
| `--recipient`      | `-r`  | `EIGENBELEG_RECIPIENT`  | yes      | Recipient of the payment                                               |
| `--type`           | `-t`  | `EIGENBELEG_TYPE`       | no       | Type of expenditure (default: `Privatentnahme` or `Privateinlage`)      |
| `--reason`         | `-g`  | `EIGENBELEG_REASON`     | yes      | Reason for the Eigenbeleg                                              |
| `--date`           | `-d`  | `EIGENBELEG_DATE`       | no       | Date as `DD.MM.YYYY` (default: today)                                  |
| `--signature`      | `-i`  | `EIGENBELEG_SIGNATURE`  | yes      | Path to signature image, PNG or JPEG                                    |
| `--header`         | `-H`  | `EIGENBELEG_HEADER`     | no       | Path to header image, PNG or JPEG                                      |
| `--output`         | `-o`  | `EIGENBELEG_OUTPUT`     | yes      | Path of the PDF file to write                                          |

### Example

```sh
eigenbeleg \
  --number 2026-001 \
  --amount -42,50 \
  --subject "Parkgebühren" \
  --recipient "Parkhaus Musterstadt" \
  --reason "Kein Beleg erhalten" \
  --signature signature.png \
  --output eigenbeleg.pdf
```
