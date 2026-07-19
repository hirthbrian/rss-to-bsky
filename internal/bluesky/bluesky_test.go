package bluesky

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	original := baseURL
	baseURL = server.URL
	t.Cleanup(func() { baseURL = original })

	return server
}

func TestLogin_Success(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.createSession" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["identifier"] != "handle.bsky.social" || body["password"] != "app-password" {
			t.Errorf("unexpected credentials: %+v", body)
		}

		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessJwt": "token-123",
			"did":       "did:plc:abc",
		})
	})

	sess, err := Login("handle.bsky.social", "app-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if sess.AccessJwt != "token-123" || sess.Did != "did:plc:abc" {
		t.Errorf("unexpected session: %+v", sess)
	}
}

func TestLogin_NonOKStatus(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	if _, err := Login("handle.bsky.social", "wrong-password"); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestSessionPost_Success(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.repo.createRecord" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Errorf("unexpected Authorization header: %q", got)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["repo"] != "did:plc:abc" {
			t.Errorf("unexpected repo: %v", body["repo"])
		}
		record, ok := body["record"].(map[string]interface{})
		if !ok || record["text"] != "hello world" {
			t.Errorf("unexpected record: %+v", body["record"])
		}
		if _, hasFacets := record["facets"]; hasFacets {
			t.Errorf("expected no facets when link is empty, got: %v", record["facets"])
		}

		w.WriteHeader(http.StatusOK)
	})

	sess := &Session{AccessJwt: "token-123", Did: "did:plc:abc"}
	if err := sess.Post("hello world", ""); err != nil {
		t.Fatalf("Post() error = %v", err)
	}
}

func TestSessionPost_NonOKStatus(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	sess := &Session{AccessJwt: "token-123", Did: "did:plc:abc"}
	if err := sess.Post("hello world", ""); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestSessionPost_IncludesLinkFacet(t *testing.T) {
	const link = "https://example.com/article"
	text := "Check this out\n" + link
	wantStart := float64(strings.Index(text, link))
	wantEnd := wantStart + float64(len(link))

	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}

		record, ok := body["record"].(map[string]interface{})
		if !ok {
			t.Fatalf("missing record in body: %+v", body)
		}

		facets, ok := record["facets"].([]interface{})
		if !ok || len(facets) != 1 {
			t.Fatalf("expected exactly one facet, got: %+v", record["facets"])
		}

		facet, ok := facets[0].(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected facet shape: %+v", facets[0])
		}

		index, ok := facet["index"].(map[string]interface{})
		if !ok || index["byteStart"] != wantStart || index["byteEnd"] != wantEnd {
			t.Errorf("unexpected facet index: %+v (want start=%v end=%v)", index, wantStart, wantEnd)
		}

		features, ok := facet["features"].([]interface{})
		if !ok || len(features) != 1 {
			t.Fatalf("expected exactly one feature, got: %+v", facet["features"])
		}
		feature, ok := features[0].(map[string]interface{})
		if !ok || feature["$type"] != "app.bsky.richtext.facet#link" || feature["uri"] != link {
			t.Errorf("unexpected feature: %+v", feature)
		}

		w.WriteHeader(http.StatusOK)
	})

	sess := &Session{AccessJwt: "token-123", Did: "did:plc:abc"}
	if err := sess.Post(text, link); err != nil {
		t.Fatalf("Post() error = %v", err)
	}
}

func TestSessionPost_LinkNotInTextOmitsFacet(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		record := body["record"].(map[string]interface{})
		if _, hasFacets := record["facets"]; hasFacets {
			t.Errorf("expected no facets when link doesn't occur in text, got: %v", record["facets"])
		}

		w.WriteHeader(http.StatusOK)
	})

	sess := &Session{AccessJwt: "token-123", Did: "did:plc:abc"}
	if err := sess.Post("hello world", "https://example.com/not-in-text"); err != nil {
		t.Fatalf("Post() error = %v", err)
	}
}
