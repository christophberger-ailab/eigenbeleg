package main

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/christophberger/eigenbeleg"
)

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"german notation", "1.234,56", 1234.56, false},
		{"plain decimal", "1234.56", 1234.56, false},
		{"negative german with euro suffix", "-42,50 €", -42.50, false},
		{"invalid", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAmount(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseAmount(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAmount(%q) unexpected error: %v", tt.input, err)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("parseAmount(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	t.Run("valid date", func(t *testing.T) {
		got, err := parseDate("2026-07-01")
		if err != nil {
			t.Fatalf("parseDate() unexpected error: %v", err)
		}
		want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("parseDate() = %v, want %v", got, want)
		}
	})

	t.Run("empty string yields zero time", func(t *testing.T) {
		got, err := parseDate("")
		if err != nil {
			t.Fatalf("parseDate() unexpected error: %v", err)
		}
		if !got.IsZero() {
			t.Errorf("parseDate(\"\") = %v, want zero time", got)
		}
	})

	t.Run("wrong format returns error", func(t *testing.T) {
		_, err := parseDate("01.07.2026")
		if err == nil {
			t.Fatal("parseDate(\"01.07.2026\") expected error, got nil")
		}
	})
}

func TestPrintUsage(t *testing.T) {
	var out strings.Builder
	printUsage(&out)
	got := out.String()
	for _, want := range []string{description, "Usage of", "--amount", "-i, --signature"} {
		if !strings.Contains(got, want) {
			t.Errorf("printUsage() output missing %q; got:\n%s", want, got)
		}
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"enter confirms", "\n", true},
		{"y confirms", "y\n", true},
		{"yes confirms", "Yes\n", true},
		{"end of input confirms", "", true},
		{"n declines", "n\n", false},
		{"anything else declines", "maybe\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			got, err := confirm(strings.NewReader(tt.input), &out)
			if err != nil {
				t.Fatalf("confirm(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("confirm(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if !strings.Contains(out.String(), "Generate PDF?") {
				t.Errorf("confirm() prompt = %q, want it to contain %q", out.String(), "Generate PDF?")
			}
		})
	}
}

func TestPrintFields(t *testing.T) {
	r := eigenbeleg.Receipt{
		Number:        "42",
		Amount:        -1234.56,
		Subject:       "Privatentnahme",
		Recipient:     "Buchhandlung Meier",
		Type:          "Privatentnahme",
		Reason:        "Kein Beleg erhalten",
		Date:          time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		SignaturePath: "sig.png",
	}
	var out strings.Builder
	printFields(&out, r, "2026-07-01 Eigenbeleg 42.pdf")
	got := out.String()
	for _, want := range []string{
		"number:    42",
		"amount:    1.234,56 € (Ausgabe)",
		"subject:   Privatentnahme",
		"recipient: Buchhandlung Meier",
		"date:      2026-07-01",
		"header:    (none)",
		"output:    2026-07-01 Eigenbeleg 42.pdf",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("printFields() output missing %q; got:\n%s", want, got)
		}
	}
}

func TestDefaultOutputPath(t *testing.T) {
	t.Run("with date", func(t *testing.T) {
		got := defaultOutputPath(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), "42")
		want := "2026-07-01 Eigenbeleg 42.pdf"
		if got != want {
			t.Errorf("defaultOutputPath() = %q, want %q", got, want)
		}
	})

	t.Run("zero date falls back to today", func(t *testing.T) {
		got := defaultOutputPath(time.Time{}, "42")
		want := time.Now().Format("2006-01-02") + " Eigenbeleg 42.pdf"
		if got != want {
			t.Errorf("defaultOutputPath() = %q, want %q", got, want)
		}
	})
}
