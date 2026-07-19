package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/robfig/cron/v3"

	"github.com/brianhirth/rss-to-bsky/internal/bluesky"
	"github.com/brianhirth/rss-to-bsky/internal/feed"
	"github.com/brianhirth/rss-to-bsky/internal/opengraph"
	"github.com/brianhirth/rss-to-bsky/internal/store"
)

// maxThumbBytes is Bluesky's upload size limit for embed thumbnail blobs.
const maxThumbBytes = 1_000_000

const defaultDBPath = "/data/posted.db"

func main() {
	rssURL := requireEnv("RSS_URL")
	handle := requireEnv("BSKY_HANDLE")
	appPassword := requireEnv("BSKY_APP_PASSWORD")

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = defaultDBPath
	}

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("opening store: %v", err)
	}
	defer st.Close()

	run := func() {
		if err := runOnce(st, rssURL, handle, appPassword); err != nil {
			log.Printf("run failed: %v", err)
		}
	}

	run()

	c := cron.New()
	if _, err := c.AddFunc("@every 15m", run); err != nil {
		log.Fatalf("scheduling cron: %v", err)
	}
	c.Start()

	select {}
}

func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("missing required env var %s", name)
	}
	return v
}

func runOnce(st *store.Store, rssURL, handle, appPassword string) error {
	items, err := feed.Fetch(rssURL)
	if err != nil {
		return err
	}

	var sess *bluesky.Session

	for _, item := range items {
		posted, err := st.AlreadyPosted(item.GUID)
		if err != nil {
			return fmt.Errorf("checking dedup for %q: %w", item.GUID, err)
		}
		if posted {
			continue
		}

		if sess == nil {
			sess, err = bluesky.Login(handle, appPassword)
			if err != nil {
				return fmt.Errorf("bluesky login: %w", err)
			}
		}

		embed := buildEmbed(sess, item)

		if err := sess.Post(item.Text, item.Link, embed); err != nil {
			return fmt.Errorf("posting %q: %w", item.GUID, err)
		}

		if err := st.MarkPosted(item.GUID); err != nil {
			return fmt.Errorf("marking posted for %q: %w", item.GUID, err)
		}

		log.Printf("posted: %s", item.GUID)
	}

	return nil
}

// buildEmbed fetches the linked article's Open Graph metadata and builds a
// link preview card for it. Link previews are best-effort: any failure
// fetching metadata or the thumbnail image is logged and the post proceeds
// without an embed, rather than failing the whole run.
func buildEmbed(sess *bluesky.Session, item feed.Item) *bluesky.ExternalEmbed {
	if item.Link == "" {
		return nil
	}

	meta, err := opengraph.Fetch(item.Link)
	if err != nil {
		log.Printf("fetching link preview for %q: %v", item.Link, err)
		return nil
	}

	title := meta.Title
	if title == "" {
		title = item.Title
	}
	if title == "" {
		title = item.Link
	}

	embed := &bluesky.ExternalEmbed{
		URI:         item.Link,
		Title:       title,
		Description: meta.Description,
	}

	if meta.ImageURL != "" {
		thumb, err := fetchAndUploadThumb(sess, meta.ImageURL)
		if err != nil {
			log.Printf("uploading link preview image for %q: %v", item.Link, err)
		} else {
			embed.Thumb = thumb
		}
	}

	return embed
}

func fetchAndUploadThumb(sess *bluesky.Session, imageURL string) (json.RawMessage, error) {
	resp, err := http.Get(imageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching thumbnail: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxThumbBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading thumbnail: %w", err)
	}
	if len(data) > maxThumbBytes {
		return nil, fmt.Errorf("thumbnail exceeds %d byte limit", maxThumbBytes)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	return sess.UploadBlob(data, mimeType)
}
