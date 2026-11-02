package handlers

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useTemplates writes minimal templates into a temp dir and points
// templatesDir at it for the duration of the test.
func useTemplates(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	old := templatesDir
	templatesDir = dir
	t.Cleanup(func() { templatesDir = old })

	oldOut := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(oldOut) })
}

func standardTemplates() map[string]string {
	return map[string]string{
		"layout.html": `<html>{{template "content" .}}</html>`,
		"400.html":    `{{define "content"}}Bad request page{{end}}`,
		"404.html":    `{{define "content"}}Not found page{{end}}`,
		"500.html":    `{{define "content"}}Server error page{{end}}`,
	}
}

func TestErrorPages(t *testing.T) {
	useTemplates(t, standardTemplates())
	tests := []struct {
		name string
		call func(w http.ResponseWriter, r *http.Request)
		code int
		body string
	}{
		{"404", NotFound, http.StatusNotFound, "Not found page"},
		{"400", BadRequest, http.StatusBadRequest, "Bad request page"},
		{"500", func(w http.ResponseWriter, r *http.Request) { InternalError(w, r, errors.New("boom")) },
			http.StatusInternalServerError, "Server error page"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.call(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
			if rec.Code != tc.code {
				t.Errorf("status = %d, want %d", rec.Code, tc.code)
			}
			if !strings.Contains(rec.Body.String(), tc.body) {
				t.Errorf("body = %q, want it to contain %q", rec.Body.String(), tc.body)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Errorf("Content-Type = %q, want text/html", ct)
			}
		})
	}
}

func TestErrorPages_MissingTemplatesFallBackToPlainText(t *testing.T) {
	useTemplates(t, nil) // empty directory: nothing to parse
	rec := httptest.NewRecorder()
	NotFound(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 even without templates", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Not Found") {
		t.Errorf("body = %q, want the plain-text fallback", rec.Body.String())
	}
}

func TestRenderPage_TemplateErrorLeavesNoPartialPage(t *testing.T) {
	files := standardTemplates()
	files["broken.html"] = `{{define "content"}}start {{.Missing.Field}} end{{end}}`
	useTemplates(t, files)

	rec := httptest.NewRecorder()
	renderPage(rec, http.StatusOK, "broken.html", struct{}{})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<html>") || strings.Contains(rec.Body.String(), "start") {
		t.Errorf("body %q contains part of the failed page", rec.Body.String())
	}
}

func TestRecover_PassesNormalRequestsThrough(t *testing.T) {
	useTemplates(t, standardTemplates())
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fine"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "fine" {
		t.Errorf("got %d %q, want 200 \"fine\"", rec.Code, rec.Body.String())
	}
}

func TestRecover_PanicBecomes500AndServerSurvives(t *testing.T) {
	useTemplates(t, standardTemplates())
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	}))
	for i := 0; i < 2; i++ { // the same wrapped handler must keep working
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("call %d: status = %d, want 500", i, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Server error page") {
			t.Errorf("call %d: body = %q, want the 500 page", i, rec.Body.String())
		}
	}
}

func TestRecover_PanicAfterPartialWriteKeepsStatusAndAddsNothing(t *testing.T) {
	useTemplates(t, standardTemplates())
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("partial"))
		panic("late panic")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (already sent)", rec.Code)
	}
	if rec.Body.String() != "partial" {
		t.Errorf("body = %q, want only the part written before the panic", rec.Body.String())
	}
}

func TestRecover_RePanicsErrAbortHandler(t *testing.T) {
	useTemplates(t, standardTemplates())
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if rec := recover(); rec != http.ErrAbortHandler {
			t.Errorf("recovered %v, want http.ErrAbortHandler to propagate", rec)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
