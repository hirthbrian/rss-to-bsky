// Package opengraph fetches a web page and extracts its Open Graph
// metadata, for building Bluesky link-preview cards.
package opengraph

import (
	"fmt"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

// Metadata holds the Open Graph tags relevant to a link preview card. Any
// field may be empty if the page doesn't define that tag — callers should
// treat that as "not available" and fall back accordingly, not as an error.
type Metadata struct {
	Title       string
	Description string
	ImageURL    string
}

func Fetch(url string) (Metadata, error) {
	resp, err := http.Get(url)
	if err != nil {
		return Metadata{}, fmt.Errorf("fetching %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("fetching %q: status %d", url, resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Metadata{}, fmt.Errorf("parsing %q: %w", url, err)
	}

	return Metadata{
		Title:       ogTag(doc, "og:title"),
		Description: ogTag(doc, "og:description"),
		ImageURL:    ogTag(doc, "og:image"),
	}, nil
}

func ogTag(doc *goquery.Document, property string) string {
	content, _ := doc.Find(`meta[property="` + property + `"]`).First().Attr("content")
	return content
}
