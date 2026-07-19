package feed

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
)

const testRSS = `<?xml version="1.0"?>
<rss version="2.0">
<channel>
<title>Test Feed</title>
<item>
<title>First Post</title>
<link>https://example.com/first</link>
<guid>guid-1</guid>
</item>
<item>
<title>Second Post</title>
<link>https://example.com/second</link>
</item>
<item>
<title></title>
<link></link>
<guid></guid>
</item>
</channel>
</rss>`

func TestFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(testRSS))
	}))
	defer server.Close()

	items, err := Fetch(server.URL)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	// The third item has neither guid nor link and should be skipped.
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(items), items)
	}

	if items[0].GUID != "guid-1" {
		t.Errorf("expected GUID %q, got %q", "guid-1", items[0].GUID)
	}
	if !strings.Contains(items[0].Text, "First Post") || !strings.Contains(items[0].Text, "https://example.com/first") {
		t.Errorf("unexpected text: %q", items[0].Text)
	}

	if items[1].GUID != "https://example.com/second" {
		t.Errorf("expected fallback GUID to be link %q, got %q", "https://example.com/second", items[1].GUID)
	}
}

func TestFetch_InvalidURL(t *testing.T) {
	if _, err := Fetch("http://127.0.0.1:0/does-not-exist"); err == nil {
		t.Fatal("expected error for unreachable feed URL")
	}
}

func TestFormatPost(t *testing.T) {
	got := formatPost(&gofeed.Item{Title: "Hello", Link: "https://example.com/x"})
	want := "Hello\nhttps://example.com/x"
	if got != want {
		t.Errorf("formatPost() = %q, want %q", got, want)
	}
}

func TestFormatPost_TruncatesTitleNotLink(t *testing.T) {
	longTitle := strings.Repeat("a", 400)
	link := "https://example.com/article"

	got := formatPost(&gofeed.Item{Title: longTitle, Link: link})

	if !strings.HasSuffix(got, link) {
		t.Errorf("expected formatted post to end with the full, untruncated link %q, got %q", link, got)
	}
	if got := []rune(got); len(got) > maxPostChars {
		t.Errorf("expected formatted post to respect maxPostChars=%d, got %d runes", maxPostChars, len(got))
	}
}

func TestFormatPost_NoLink(t *testing.T) {
	got := formatPost(&gofeed.Item{Title: "Hello"})
	if got != "Hello" {
		t.Errorf("formatPost() = %q, want %q", got, "Hello")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"under limit", "short", 10, "short"},
		{"exact limit", "exact", 5, "exact"},
		{"over limit", "this is too long", 7, "this is"},
		{"multi-byte runes", strings.Repeat("é", 10), 5, strings.Repeat("é", 5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.in, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}
