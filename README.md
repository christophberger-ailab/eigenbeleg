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

Running `eigenbeleg` without any arguments prints the usage, same as
`eigenbeleg --help`.

| Flag              | Short | Env var                | Required | Description                                                          |
| ------------------ | ----- | ----------------------- | -------- | ---------------------------------------------------------------------- |
| `--number`         | `-n`  | `EIGENBELEG_NUMBER`     | yes      | Receipt number                                                        |
| `--amount`         | `-a`  | `EIGENBELEG_AMOUNT`     | yes      | Amount, e.g. `1.234,56` or `-42.50`; negative for expenses             |
| `--subject`        | `-s`  | `EIGENBELEG_SUBJECT`    | no       | Subject of the transaction (default: `Privatentnahme` or `Privateinlage`) |
| `--recipient`      | `-r`  | `EIGENBELEG_RECIPIENT`  | yes      | Recipient of the payment                                               |
| `--type`           | `-t`  | `EIGENBELEG_TYPE`       | no       | Type of expenditure (default: `Privatentnahme` or `Privateinlage`)      |
| `--reason`         | `-g`  | `EIGENBELEG_REASON`     | yes      | Reason for the Eigenbeleg                                              |
| `--date`           | `-d`  | `EIGENBELEG_DATE`       | no       | Date as `YYYY-MM-DD` (default: today)                                  |
| `--signature`      | `-i`  | `EIGENBELEG_SIGNATURE`  | yes      | Path to signature image, PNG or JPEG                                    |
| `--header`         | `-H`  | `EIGENBELEG_HEADER`     | no       | Path to header image, PNG or JPEG                                      |
| `--output`         | `-o`  | `EIGENBELEG_OUTPUT`     | no       | Path of the PDF file to write (default: `./<date> Eigenbeleg <number>.pdf`) |

### Example

```sh
eigenbeleg \
  --number 2026-001 \
  --amount -42,50 \
  --subject "Parkgebühren" \
  --recipient "Parkhaus Musterstadt" \
  --reason "Kein Beleg erhalten" \
  --signature signature.png
```

After validating the input, the tool lists all fields with inputs and
defaults filled in and asks for confirmation before generating the PDF;
press Enter (or answer `y`) to proceed.

This writes the PDF to `./<date> Eigenbeleg <number>.pdf` (e.g.
`2026-001` on 2026-07-19 yields `2026-07-19 Eigenbeleg 2026-001.pdf`);
pass `--output` to choose a different path.

## Web UI

```sh
eigenbeleg web --signature signature.png
```

starts a local web UI at <http://localhost:8080/>. It lists all saved
Eigenbelege in chronological order on the left (the newest stay in view),
and shows the selected one in an edit form. From there you can create a new
Eigenbeleg, save changes, and download the PDF. An Eigenbeleg created by
mistake can be hidden ("Ausblenden"); hidden ones stay in the store and can be
shown and restored via "Ausgeblendete anzeigen".

| Flag          | Env var             | Default             | Description                                              |
| ------------- | ------------------- | ------------------- | -------------------------------------------------------- |
| `--addr`      | `EIGENBELEG_ADDR`   | `localhost:8080`    | Address to listen on                                     |
| `--store`     | `EIGENBELEG_STORE`  | `eigenbelege.json`  | JSON file that stores the Eigenbelege                    |
| `--signature` |                     |                     | Default signature image path for new Eigenbelege         |
| `--header`    |                     |                     | Default header image path for new Eigenbelege            |

Image paths are read on the machine running `eigenbeleg web`, so keep the
default listen address on localhost. The UI uses [Pico CSS](https://picocss.com)
and [htmx](https://htmx.org), both embedded in the binary, so it works offline.
