package main

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func newTestApp(t *testing.T) (*webApp, string) {
	t.Helper()
	dir := t.TempDir()
	sig := filepath.Join(dir, "sig.png")
	f, err := os.Create(sig)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewGray(image.Rect(0, 0, 20, 10))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	s, err := openStore(filepath.Join(dir, "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	app, err := newWebApp(s, formData{Signature: sig})
	if err != nil {
		t.Fatal(err)
	}
	return app, sig
}

func do(t *testing.T, h http.Handler, method, target string, form url.Values, htmx bool) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if htmx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func validForm(sig string) url.Values {
	return url.Values{
		"number":    {"2026-001"},
		"amount":    {"-42,50"},
		"recipient": {"Parkhaus Musterstadt"},
		"reason":    {"Kein Beleg erhalten"},
		"date":      {"2026-07-19"},
		"signature": {sig},
		"action":    {"save"},
	}
}

func TestWebCreateEditHide(t *testing.T) {
	app, sig := newTestApp(t)
	h := app.routes()

	// The empty index shows a blank form with the configured signature.
	res := do(t, h, "GET", "/", nil, false)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "Neuer Eigenbeleg") || !strings.Contains(res.Body.String(), sig) {
		t.Fatalf("GET / = %d, body:\n%s", res.Code, res.Body)
	}

	// Missing required fields are reported, nothing is stored.
	res = do(t, h, "POST", "/receipts", url.Values{"amount": {"abc"}}, true)
	if !strings.Contains(res.Body.String(), `aria-invalid="true"`) || !strings.Contains(res.Body.String(), "Ungültiger Betrag") {
		t.Errorf("invalid form: expected field errors, got:\n%s", res.Body)
	}
	if n := len(app.store.list()); n != 0 {
		t.Fatalf("invalid form stored %d records", n)
	}

	// A valid form is stored.
	res = do(t, h, "POST", "/receipts", validForm(sig), true)
	if got := res.Header().Get("HX-Push-Url"); got != "/receipts/1" {
		t.Errorf("HX-Push-Url = %q, want /receipts/1", got)
	}
	if !strings.Contains(res.Body.String(), `hx-swap-oob="true"`) {
		t.Errorf("htmx response lacks out-of-band list update")
	}
	rec, err := app.store.get(1)
	if err != nil || rec.Amount != -42.5 || rec.Recipient != "Parkhaus Musterstadt" {
		t.Fatalf("stored record = %+v, %v", rec, err)
	}

	// The next new form suggests the following number.
	res = do(t, h, "GET", "/new", nil, true)
	if !strings.Contains(res.Body.String(), `value="2026-002"`) {
		t.Errorf("new form does not suggest number 2026-002:\n%s", res.Body)
	}

	// Copying shows an unsaved form with the fields of the original, the
	// next number, and today's date.
	res = do(t, h, "GET", "/receipts/1/copy", nil, true)
	body := res.Body.String()
	today := time.Now().Format("2006-01-02")
	for _, want := range []string{"Neuer Eigenbeleg", "Kopie von Eigenbeleg 2026-001", `value="2026-002"`, `value="` + today + `"`, `value="Parkhaus Musterstadt"`, "Kein Beleg erhalten", `hx-post="/receipts"`} {
		if !strings.Contains(body, want) {
			t.Errorf("copy form lacks %q:\n%s", want, body)
		}
	}
	if n := len(app.store.list()); n != 1 {
		t.Errorf("copying stored a record: %d records", n)
	}
	if res := do(t, h, "GET", "/receipts/99/copy", nil, false); res.Code != http.StatusNotFound {
		t.Errorf("GET copy of unknown receipt = %d, want 404", res.Code)
	}

	// Saving with the PDF button updates the record and triggers the download.
	form := validForm(sig)
	form.Set("recipient", "Parkhaus Nord")
	form.Set("action", "pdf")
	res = do(t, h, "POST", "/receipts/1", form, true)
	if got := res.Header().Get("HX-Trigger"); !strings.Contains(got, "/receipts/1/pdf") {
		t.Errorf("HX-Trigger = %q, want download of /receipts/1/pdf", got)
	}
	if rec, _ := app.store.get(1); rec.Recipient != "Parkhaus Nord" {
		t.Errorf("recipient not updated: %+v", rec)
	}

	res = do(t, h, "GET", "/receipts/1/pdf", nil, false)
	if res.Code != http.StatusOK || !bytes.HasPrefix(res.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("GET pdf = %d, %.40q", res.Code, res.Body)
	}
	if cd := res.Header().Get("Content-Disposition"); !strings.Contains(cd, "2026-07-19 Eigenbeleg 2026-001.pdf") {
		t.Errorf("Content-Disposition = %q", cd)
	}

	// Hiding keeps the record but marks it hidden; unhiding restores it.
	res = do(t, h, "POST", "/receipts/1/hide", nil, true)
	if !strings.Contains(res.Body.String(), "ausgeblendet") || !strings.Contains(res.Body.String(), "/receipts/1/unhide") {
		t.Errorf("hide response lacks notice with undo:\n%s", res.Body)
	}
	if rec, _ := app.store.get(1); !rec.Hidden {
		t.Errorf("record not hidden")
	}
	do(t, h, "POST", "/receipts/1/unhide", nil, true)
	if rec, _ := app.store.get(1); rec.Hidden {
		t.Errorf("record still hidden")
	}

	// Data survives reopening the store.
	s, err := openStore(app.store.path)
	if err != nil {
		t.Fatal(err)
	}
	if recs := s.list(); len(recs) != 1 || recs[0].Recipient != "Parkhaus Nord" || s.NextID != 2 {
		t.Errorf("reopened store = %+v, next ID %d", recs, s.NextID)
	}

	if res := do(t, h, "GET", "/receipts/99", nil, false); res.Code != http.StatusNotFound {
		t.Errorf("GET unknown receipt = %d, want 404", res.Code)
	}
}

func TestStoreListChronological(t *testing.T) {
	s, err := openStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"2026-03-01", "2026-01-15", "2026-03-01"} {
		if _, err := s.create(record{Date: d}); err != nil {
			t.Fatal(err)
		}
	}
	var got []int
	for _, r := range s.list() {
		got = append(got, r.ID)
	}
	if want := []int{2, 1, 3}; !slices.Equal(got, want) {
		t.Errorf("list order = %v, want %v", got, want)
	}
}

func TestStoreNextNumber(t *testing.T) {
	s := &store{}
	for _, tt := range []struct{ last, want string }{
		{"2026-007", "2026-008"},
		{"2026-099", "2026-100"},
		{"42", "43"},
		{"ABC", ""},
	} {
		s.Records = []record{{Number: tt.last}}
		if got := s.nextNumber(); got != tt.want {
			t.Errorf("nextNumber() after %q = %q, want %q", tt.last, got, tt.want)
		}
	}
}
