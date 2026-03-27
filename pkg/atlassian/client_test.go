package atlassian

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetch(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"key": "value"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "key")
	content, err := client.Fetch(server.URL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if content != `{"key": "value"}` {
		t.Errorf("expected content to be '{\"key\": \"value\"}', got %s", content)
	}
}
