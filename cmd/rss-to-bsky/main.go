package main

import (
	"fmt"
	"log"
	"os"

	"github.com/robfig/cron/v3"

	"github.com/brianhirth/rss-to-bsky/internal/bluesky"
	"github.com/brianhirth/rss-to-bsky/internal/feed"
	"github.com/brianhirth/rss-to-bsky/internal/store"
)

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

		if err := sess.Post(item.Text); err != nil {
			return fmt.Errorf("posting %q: %w", item.GUID, err)
		}

		if err := st.MarkPosted(item.GUID); err != nil {
			return fmt.Errorf("marking posted for %q: %w", item.GUID, err)
		}

		log.Printf("posted: %s", item.GUID)
	}

	return nil
}
