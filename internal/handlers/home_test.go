package handlers

import (
	"net/http"
	"strings"
	"testing"

	"groupie-tracker/internal/models"
	"groupie-tracker/internal/store"
)

// useRealTemplates points renderPage at the real web/templates directory.
func useRealTemplates(t *testing.T) {
	t.Helper()
	old := templatesDir
	templatesDir = "../../web/templates"
	t.Cleanup(func() { templatesDir = old })
}

func homeServer(artists ...models.Artist) *Server {
	return &Server{Store: &store.Store{Artists: artists}}
}

func TestHome_RendersOneCardPerArtist(t *testing.T) {
	useRealTemplates(t)
	srv := homeServer(
		models.Artist{ID: 1, Name: "Queen", Image: "http://img/queen.jpg", CreationDate: 1970, Members: []string{"a", "b", "c", "d"}},
		models.Artist{ID: 2, Name: "Solo Act", Image: "http://img/solo.jpg", CreationDate: 1999, Members: []string{"x"}},
	)

	rec := get(http.HandlerFunc(srv.HomeHandler), "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Queen", "Solo Act", "Since 1970", "4 members", "1 member<", `href="/artist/1"`, `href="/artist/2"`, "2 artists",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page is missing %q", want)
		}
	}
	if n := strings.Count(body, `class="card"`); n != 2 {
		t.Errorf("cards = %d, want 2", n)
	}
}

func TestHome_EscapesArtistNames(t *testing.T) {
	useRealTemplates(t)
	srv := homeServer(models.Artist{ID: 1, Name: `<script>alert(1)</script>`, Members: []string{"a"}})

	body := get(http.HandlerFunc(srv.HomeHandler), "/").Body.String()

	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("artist name was not HTML-escaped")
	}
}

func TestHome_NoArtistsStillRenders(t *testing.T) {
	useRealTemplates(t)
	rec := get(http.HandlerFunc(homeServer().HomeHandler), "/")

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "0 artists") {
		t.Errorf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestRealErrorTemplatesRender(t *testing.T) {
	useRealTemplates(t)
	for _, tc := range []struct {
		name   string
		status int
		h      http.HandlerFunc
		want   string
	}{
		{"404", http.StatusNotFound, NotFound, "Page not found"},
		{"400", http.StatusBadRequest, BadRequest, "Bad request"},
	} {
		rec := get(tc.h, "/x")
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: status = %d, body = %q", tc.name, rec.Code, rec.Body.String())
		}
	}
}
