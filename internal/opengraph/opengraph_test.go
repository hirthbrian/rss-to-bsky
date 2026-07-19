package opengraph

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetch(t *testing.T) {
	const html = `<!DOCTYPE html>
<html>
<head>
<meta property="og:title" content="An Article">
<meta property="og:description" content="A description of the article.">
<meta property="og:image" content="https://example.com/thumb.jpg">
</head>
<body></body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	got, err := Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	want := Metadata{
		Title:       "An Article",
		Description: "A description of the article.",
		ImageURL:    "https://example.com/thumb.jpg",
	}
	if got != want {
		t.Errorf("Fetch() = %+v, want %+v", got, want)
	}
}

func TestFetch_MissingTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head></head><body></body></html>`))
	}))
	defer server.Close()

	got, err := Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got != (Metadata{}) {
		t.Errorf("expected zero-value Metadata for a page with no OG tags, got %+v", got)
	}
}

func TestFetch_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	if _, err := Fetch(server.URL); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestFetch_InvalidURL(t *testing.T) {
	if _, err := Fetch("http://127.0.0.1:0/does-not-exist"); err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}
