package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/christophberger/eigenbeleg"
)

//go:embed web/templates/*.html
var templateFS embed.FS

//go:embed web/static
var staticFS embed.FS

// formData holds the form fields of an Eigenbeleg as entered by the user.
type formData struct {
	Number, Amount, Subject, Recipient, Type, Reason, Date, Signature, Header string
}

// formView is the main view: the edit form for a new or existing record.
type formView struct {
	ID     int // 0 for a new, not yet saved record
	Hidden bool
	Form   formData
	Errors map[string]string
	// Notice is a status message shown above the form.
	Notice string
	// UndoHide, if non-zero, is the ID of a record that was just hidden;
	// the notice then offers to restore it.
	UndoHide int
}

// field is the view of a single form field.
type field struct {
	Name, Label, Value, Placeholder, Error string
	Required                               bool
}

func newField(name, label, value, placeholder string, errs map[string]string, required bool) field {
	return field{Name: name, Label: label, Value: value, Placeholder: placeholder, Error: errs[name], Required: required}
}

type pageView struct {
	Records  []record
	Selected int
	Main     formView
}

type webApp struct {
	store    *store
	tmpl     *template.Template
	defaults formData // signature and header paths for new records
}

func runWeb(addr, storePath string) error {
	s, err := openStore(storePath)
	if err != nil {
		return err
	}
	app, err := newWebApp(s, formData{Signature: *signature, Header: *header})
	if err != nil {
		return err
	}
	abs, _ := filepath.Abs(storePath)
	log.Printf("Eigenbelege are stored in %s", abs)
	log.Printf("Web UI listening on http://%s/", addr)
	return http.ListenAndServe(addr, app.routes())
}

func newWebApp(s *store, defaults formData) (*webApp, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"displayDate":   displayDate,
		"displayAmount": displayAmount,
		"field":         newField,
	}).ParseFS(templateFS, "web/templates/*.html")
	if err != nil {
		return nil, err
	}
	return &webApp{store: s, tmpl: tmpl, defaults: defaults}, nil
}

func (a *webApp) routes() http.Handler {
	static, _ := fs.Sub(staticFS, "web")
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServerFS(static))
	mux.HandleFunc("GET /{$}", a.index)
	mux.HandleFunc("GET /new", a.newForm)
	mux.HandleFunc("POST /receipts", a.save)
	mux.HandleFunc("GET /receipts/{id}", a.show)
	mux.HandleFunc("POST /receipts/{id}", a.save)
	mux.HandleFunc("GET /receipts/{id}/pdf", a.pdf)
	mux.HandleFunc("POST /receipts/{id}/hide", a.hide)
	mux.HandleFunc("POST /receipts/{id}/unhide", a.unhide)
	return mux
}

// index shows the newest visible record, or an empty form if there is none.
func (a *webApp) index(w http.ResponseWriter, r *http.Request) {
	if rec, ok := a.newestVisible(); ok {
		a.render(w, r, formView{ID: rec.ID, Form: toForm(rec)})
		return
	}
	a.render(w, r, a.blankForm())
}

func (a *webApp) newForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, a.blankForm())
}

func (a *webApp) show(w http.ResponseWriter, r *http.Request) {
	rec, ok := a.record(w, r)
	if !ok {
		return
	}
	a.render(w, r, formView{ID: rec.ID, Hidden: rec.Hidden, Form: toForm(rec)})
}

// save creates or updates a record from the submitted form. If the form was
// submitted with the PDF button, the browser is told to download the PDF
// once the record is saved.
func (a *webApp) save(w http.ResponseWriter, r *http.Request) {
	var id int
	if r.PathValue("id") != "" {
		existing, ok := a.record(w, r)
		if !ok {
			return
		}
		id = existing.ID
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fd := formData{
		Number:    strings.TrimSpace(r.PostFormValue("number")),
		Amount:    strings.TrimSpace(r.PostFormValue("amount")),
		Subject:   strings.TrimSpace(r.PostFormValue("subject")),
		Recipient: strings.TrimSpace(r.PostFormValue("recipient")),
		Type:      strings.TrimSpace(r.PostFormValue("type")),
		Reason:    strings.TrimSpace(r.PostFormValue("reason")),
		Date:      strings.TrimSpace(r.PostFormValue("date")),
		Signature: strings.TrimSpace(r.PostFormValue("signature")),
		Header:    strings.TrimSpace(r.PostFormValue("header")),
	}
	rec, errs := fromForm(fd)
	view := formView{ID: id, Form: fd, Errors: errs}
	if len(errs) > 0 {
		view.Notice = "Bitte die markierten Felder korrigieren."
		if id != 0 {
			cur, _ := a.store.get(id)
			view.Hidden = cur.Hidden
		}
		a.render(w, r, view)
		return
	}

	rec.ID = id
	var err error
	if id == 0 {
		rec, err = a.store.create(rec)
	} else {
		err = a.store.update(rec)
		rec, _ = a.store.get(id)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("/receipts/%d", rec.ID)
	wantPDF := r.PostFormValue("action") == "pdf"
	if !isHTMX(r) {
		if wantPDF {
			url += "/pdf"
		}
		http.Redirect(w, r, url, http.StatusSeeOther)
		return
	}
	w.Header().Set("HX-Push-Url", url)
	view = formView{ID: rec.ID, Hidden: rec.Hidden, Form: toForm(rec), Notice: "Gespeichert."}
	if wantPDF {
		trigger, _ := json.Marshal(map[string]string{"download": url + "/pdf"})
		w.Header().Set("HX-Trigger", string(trigger))
		view.Notice = "Gespeichert. Das PDF wird heruntergeladen."
	}
	a.render(w, r, view)
}

// pdf generates the PDF of a stored record and sends it as a download.
func (a *webApp) pdf(w http.ResponseWriter, r *http.Request) {
	rec, ok := a.record(w, r)
	if !ok {
		return
	}
	receipt, err := toReceipt(rec)
	if err == nil {
		var data []byte
		if data, err = eigenbeleg.Generate(receipt); err == nil {
			name := defaultOutputPath(receipt.Date, receipt.Number)
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
			w.Write(data)
			return
		}
	}
	http.Error(w, "Generating PDF failed: "+err.Error(), http.StatusUnprocessableEntity)
}

// hide hides a record and shows the newest visible one instead, with an
// option to undo.
func (a *webApp) hide(w http.ResponseWriter, r *http.Request) {
	rec, ok := a.record(w, r)
	if !ok {
		return
	}
	if err := a.store.setHidden(rec.ID, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view := a.blankForm()
	url := "/new"
	if next, ok := a.newestVisible(); ok {
		view = formView{ID: next.ID, Form: toForm(next)}
		url = fmt.Sprintf("/receipts/%d", next.ID)
	}
	view.Notice = fmt.Sprintf("Eigenbeleg %s wurde ausgeblendet.", rec.Number)
	view.UndoHide = rec.ID
	a.redirectOrRender(w, r, url, view)
}

func (a *webApp) unhide(w http.ResponseWriter, r *http.Request) {
	rec, ok := a.record(w, r)
	if !ok {
		return
	}
	if err := a.store.setHidden(rec.ID, false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view := formView{ID: rec.ID, Form: toForm(rec), Notice: "Eigenbeleg wiederhergestellt."}
	a.redirectOrRender(w, r, fmt.Sprintf("/receipts/%d", rec.ID), view)
}

// redirectOrRender renders view for htmx requests, pushing url to the
// browser history, and redirects plain form submissions to url.
func (a *webApp) redirectOrRender(w http.ResponseWriter, r *http.Request, url string, view formView) {
	if !isHTMX(r) {
		http.Redirect(w, r, url, http.StatusSeeOther)
		return
	}
	w.Header().Set("HX-Push-Url", url)
	a.render(w, r, view)
}

// render writes the full page, or for htmx requests only the main view plus
// an out-of-band update of the list.
func (a *webApp) render(w http.ResponseWriter, r *http.Request, view formView) {
	page := pageView{Records: a.store.list(), Selected: view.ID, Main: view}
	name := "page"
	if isHTMX(r) && r.Header.Get("HX-History-Restore-Request") != "true" {
		name = "fragment"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, name, page); err != nil {
		log.Printf("rendering %s: %v", name, err)
	}
}

// record looks up the record named by the {id} path value, writing a 404
// response if there is none.
func (a *webApp) record(w http.ResponseWriter, r *http.Request) (record, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err == nil {
		var rec record
		if rec, err = a.store.get(id); err == nil {
			return rec, true
		}
	}
	http.NotFound(w, r)
	return record{}, false
}

func (a *webApp) newestVisible() (record, bool) {
	recs := a.store.list()
	for i := len(recs) - 1; i >= 0; i-- {
		if !recs[i].Hidden {
			return recs[i], true
		}
	}
	return record{}, false
}

func (a *webApp) blankForm() formView {
	fd := a.defaults
	fd.Number = a.store.nextNumber()
	fd.Date = time.Now().Format("2006-01-02")
	return formView{Form: fd}
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// fromForm validates the form fields and converts them to a record. Errors
// are keyed by field name.
func fromForm(fd formData) (record, map[string]string) {
	errs := map[string]string{}
	for _, f := range []struct{ name, value string }{
		{"number", fd.Number},
		{"amount", fd.Amount},
		{"recipient", fd.Recipient},
		{"reason", fd.Reason},
		{"signature", fd.Signature},
	} {
		if f.value == "" {
			errs[f.name] = "Pflichtfeld"
		}
	}
	var amount float64
	if fd.Amount != "" {
		var err error
		if amount, err = parseAmount(fd.Amount); err != nil {
			errs["amount"] = "Ungültiger Betrag, z.B. 1.234,56 oder -42,50"
		}
	}
	if fd.Date == "" {
		fd.Date = time.Now().Format("2006-01-02")
	} else if _, err := parseDate(fd.Date); err != nil {
		errs["date"] = "Ungültiges Datum, erwartet JJJJ-MM-TT"
	}
	for _, img := range []struct{ name, path string }{
		{"signature", fd.Signature},
		{"header", fd.Header},
	} {
		if img.path == "" {
			continue
		}
		if msg := checkImage(img.path); msg != "" {
			errs[img.name] = msg
		}
	}
	if len(errs) > 0 {
		return record{}, errs
	}
	return record{
		Number:    fd.Number,
		Amount:    amount,
		Subject:   fd.Subject,
		Recipient: fd.Recipient,
		Type:      fd.Type,
		Reason:    fd.Reason,
		Date:      fd.Date,
		Signature: fd.Signature,
		Header:    fd.Header,
	}, nil
}

// checkImage returns an error message if path is not an existing PNG or
// JPEG file.
func checkImage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
	default:
		return "Nur PNG- oder JPEG-Bilder"
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "Datei nicht gefunden"
		}
		return err.Error()
	}
	return ""
}

func toForm(rec record) formData {
	return formData{
		Number:    rec.Number,
		Amount:    strings.Replace(strconv.FormatFloat(rec.Amount, 'f', 2, 64), ".", ",", 1),
		Subject:   rec.Subject,
		Recipient: rec.Recipient,
		Type:      rec.Type,
		Reason:    rec.Reason,
		Date:      rec.Date,
		Signature: rec.Signature,
		Header:    rec.Header,
	}
}

func toReceipt(rec record) (eigenbeleg.Receipt, error) {
	date, err := parseDate(rec.Date)
	if err != nil {
		return eigenbeleg.Receipt{}, err
	}
	receipt := eigenbeleg.Receipt{
		Number:        rec.Number,
		Amount:        rec.Amount,
		Subject:       rec.Subject,
		Recipient:     rec.Recipient,
		Type:          rec.Type,
		Reason:        rec.Reason,
		Date:          date,
		SignaturePath: rec.Signature,
		HeaderPath:    rec.Header,
	}
	err = receipt.Validate()
	return receipt, err
}

// displayDate turns YYYY-MM-DD into DD.MM.YYYY.
func displayDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("02.01.2006")
}

// displayAmount formats an amount in German notation, with a minus sign for
// expenses.
func displayAmount(v float64) string {
	if v < 0 {
		return "−" + eigenbeleg.FormatAmount(v)
	}
	return eigenbeleg.FormatAmount(v)
}
