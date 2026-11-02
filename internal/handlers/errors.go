package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"runtime/debug"
)

// templatesDir is a variable so tests can point it at a temporary directory.
var templatesDir = "web/templates"

// renderPage renders layout.html together with the given page template and
// writes the result with the given status code.
//
// Template contract: layout.html contains {{template "content" .}} and every
// page file defines {{define "content"}}...{{end}}. If you add more blocks to
// the layout (for example "title"), every page file must define them too.
//
// The page is rendered into a buffer first and written only on success. That
// way a template error can never leave a half-written page behind, and we can
// still send a clean 500. If rendering fails we fall back to plain text, so
// the error pages themselves can never take the server down.
func renderPage(w http.ResponseWriter, status int, page string, data any) {
	tmpl, err := template.ParseFiles(
		filepath.Join(templatesDir, "layout.html"),
		filepath.Join(templatesDir, page),
	)
	if err != nil {
		log.Printf("cannot parse templates for %s: %v", page, err)
		plainError(w, status)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		log.Printf("cannot render %s: %v", page, err)
		if status < http.StatusInternalServerError {
			status = http.StatusInternalServerError
		}
		plainError(w, status)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The status must be written BEFORE the body: after the first body byte
	// Go locks the status at 200.
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func plainError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// NotFound renders the custom 404 page.
func NotFound(w http.ResponseWriter, r *http.Request) {
	renderPage(w, http.StatusNotFound, "404.html", nil)
}

// BadRequest renders the custom 400 page.
func BadRequest(w http.ResponseWriter, r *http.Request) {
	renderPage(w, http.StatusBadRequest, "400.html", nil)
}

// InternalError logs the error and renders the custom 500 page.
func InternalError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("500 %s %s: %v", r.Method, r.URL.Path, err)
	renderPage(w, http.StatusInternalServerError, "500.html", nil)
}

// trackingWriter remembers whether anything has been written to the client.
type trackingWriter struct {
	http.ResponseWriter
	wrote bool
}

func (t *trackingWriter) WriteHeader(code int) {
	t.wrote = true
	t.ResponseWriter.WriteHeader(code)
}

func (t *trackingWriter) Write(b []byte) (int, error) {
	t.wrote = true
	return t.ResponseWriter.Write(b)
}

// Recover turns a panic in any handler into a 500 response instead of
// killing the connection, so the server keeps serving other requests.
//
// If the handler already started writing its response, the status code is
// already on the wire and cannot be changed (a second WriteHeader is ignored
// by net/http), so in that case we only log the panic.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tw := &trackingWriter{ResponseWriter: w}
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec) // net/http's own way of aborting a response: pass it on
			}
			log.Printf("panic serving %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
			if tw.wrote {
				return
			}
			InternalError(w, r, fmt.Errorf("panic: %v", rec))
		}()
		next.ServeHTTP(tw, r)
	})
}
