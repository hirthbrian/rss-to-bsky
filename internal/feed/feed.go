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
		})
	}
	return items, nil
}

func formatPost(item *gofeed.Item) string {
	text := item.Title
	if item.Link != "" {
		text = fmt.Sprintf("%s\n%s", text, item.Link)
	}
	return truncate(text, maxPostChars)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
