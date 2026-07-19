// Package feed fetches an RSS feed and reduces each entry to what's needed
// to post it to Bluesky.
package feed

import (
	"fmt"

	"github.com/mmcdole/gofeed"
)

const maxPostChars = 280

type Item struct {
	GUID string
	Text string
	Link string
}

func Fetch(url string) ([]Item, error) {
	fp := gofeed.NewParser()
	parsed, err := fp.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parsing feed: %w", err)
	}

	items := make([]Item, 0, len(parsed.Items))
	for _, raw := range parsed.Items {
		guid := raw.GUID
		if guid == "" {
			guid = raw.Link
		}
		if guid == "" {
			continue
		}
		items = append(items, Item{
			GUID: guid,
			Text: formatPost(raw),
			Link: raw.Link,
		})
	}
	return items, nil
}

// formatPost builds the post text, truncating only the title so the link
// (if present) always survives intact. A partially truncated URL would be
// both broken and unmatchable against the rich-text facet Bluesky needs to
// render it as a clickable link.
func formatPost(item *gofeed.Item) string {
	if item.Link == "" {
		return truncate(item.Title, maxPostChars)
	}

	budget := maxPostChars - len([]rune(item.Link)) - 1 // -1 for the separating newline
	if budget < 0 {
		budget = 0
	}

	return fmt.Sprintf("%s\n%s", truncate(item.Title, budget), item.Link)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
