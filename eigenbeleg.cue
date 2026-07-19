// Package eigenbeleg defines the configuration schema for the eigenbeleg
// command. Every field corresponds to a command line flag of the same name
// (or the environment variable EIGENBELEG_<FLAGNAME>).
package eigenbeleg

#Config: {
	// receipt number (required)
	number!: string & !=""

	// amount, e.g. 1.234,56 or -42.50; negative for expenses (required)
	// German notation (thousands dot, decimal comma) or plain decimal,
	// with an optional Euro suffix.
	amount!: string & =~"^-?([0-9]{1,3}(\\.[0-9]{3})*,[0-9]+|[0-9]+(\\.[0-9]+)?)\\s*€?$"

	// subject of the transaction (required)
	subject!: string & !=""

	// recipient of the payment (required)
	recipient!: string & !=""

	// type of expenditure (default: Privatentnahme or Privateinlage,
	// chosen by the sign of amount)
	type?: string

	// reason for the Eigenbeleg (required)
	reason!: string & !=""

	// date as DD.MM.YYYY (default: today)
	date?: string & =~"^[0-9]{2}\\.[0-9]{2}\\.[0-9]{4}$"

	// path to signature image, PNG or JPEG (required)
	signature!: string & =~"(?i)\\.(png|jpe?g)$"

	// path to header image, PNG or JPEG
	header?: string & =~"(?i)\\.(png|jpe?g)$"

	// path of the PDF file to write (required)
	output!: string & =~"(?i)\\.pdf$"
}
