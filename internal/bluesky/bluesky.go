// Package bluesky is a minimal AT Protocol HTTP client: enough to log in
// and create post records. No SDK.
package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const baseURL = "https://bsky.social"

// Session holds the credentials returned by com.atproto.server.createSession.
type Session struct {
	AccessJwt string `json:"accessJwt"`
	Did       string `json:"did"`
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

// Post creates a single app.bsky.feed.post record.
func (s *Session) Post(text string) error {
	record := map[string]interface{}{
		"collection": "app.bsky.feed.post",
		"repo":       s.Did,
		"record": map[string]interface{}{
			"$type":     "app.bsky.feed.post",
			"text":      text,
			"createdAt": time.Now().UTC().Format(time.RFC3339),
		},
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
