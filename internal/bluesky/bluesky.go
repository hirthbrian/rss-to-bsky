// Package bluesky is a minimal AT Protocol HTTP client: enough to log in
// and create post records. No SDK.
package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// baseURL is a var (not a const) so tests can point it at a mock server.
var baseURL = "https://bsky.social"

// Session holds the credentials returned by com.atproto.server.createSession.
type Session struct {
	AccessJwt string `json:"accessJwt"`
	Did       string `json:"did"`
}

// ExternalEmbed describes an app.bsky.embed.external link-preview card
// attached to a post. Thumb is the blob reference returned by UploadBlob,
// or nil if the card has no thumbnail image.
type ExternalEmbed struct {
	URI         string
	Title       string
	Description string
	Thumb       json.RawMessage
}

func Login(handle, appPassword string) (*Session, error) {
	body, err := json.Marshal(map[string]string{
		"identifier": handle,
		"password":   appPassword,
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(
		baseURL+"/xrpc/com.atproto.server.createSession",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("createSession returned status %d", resp.StatusCode)
	}

	var sess Session
	if err := json.NewDecoder(resp.Body).Decode(&sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// Post creates a single app.bsky.feed.post record. If link is non-empty and
// occurs in text, it's marked with a rich-text facet so Bluesky renders it
// as a clickable link instead of plain text. If embed is non-nil, a link
// preview card is attached to the post.
func (s *Session) Post(text, link string, embed *ExternalEmbed) error {
	post := map[string]interface{}{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	// Facet byte offsets are UTF-8 byte indices into text; Go string
	// indexing is already byte-based, so no rune conversion is needed.
	if link != "" {
		if start := strings.Index(text, link); start >= 0 {
			post["facets"] = []map[string]interface{}{
				{
					"index": map[string]int{
						"byteStart": start,
						"byteEnd":   start + len(link),
					},
					"features": []map[string]interface{}{
						{
							"$type": "app.bsky.richtext.facet#link",
							"uri":   link,
						},
					},
				},
			}
		}
	}

	if embed != nil {
		external := map[string]interface{}{
			"uri":         embed.URI,
			"title":       embed.Title,
			"description": embed.Description,
		}
		if len(embed.Thumb) > 0 {
			external["thumb"] = embed.Thumb
		}
		post["embed"] = map[string]interface{}{
			"$type":    "app.bsky.embed.external",
			"external": external,
		}
	}

	record := map[string]interface{}{
		"collection": "app.bsky.feed.post",
		"repo":       s.Did,
		"record":     post,
	}

	body, err := json.Marshal(record)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/xrpc/com.atproto.repo.createRecord",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.AccessJwt)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("createRecord returned status %d", resp.StatusCode)
	}

	return nil
}

// UploadBlob uploads binary data (e.g. a link preview image) via
// com.atproto.repo.uploadBlob and returns the resulting blob reference,
// suitable for use as ExternalEmbed.Thumb.
func (s *Session) UploadBlob(data []byte, mimeType string) (json.RawMessage, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/xrpc/com.atproto.repo.uploadBlob",
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("Authorization", "Bearer "+s.AccessJwt)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("uploadBlob returned status %d", resp.StatusCode)
	}

	var result struct {
		Blob json.RawMessage `json:"blob"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Blob, nil
}
