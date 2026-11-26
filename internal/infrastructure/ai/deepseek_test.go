package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	helpapp "github.com/temuin/backend/internal/application/help"
)

func TestDeepSeekRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}
		var request deepSeekRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Stream || request.Thinking["type"] != "disabled" || request.ResponseFormat["type"] != "json_object" || request.Model != "deepseek/deepseek-v4-1-flash" {
			t.Errorf("unexpected request options: %#v", request)
		}
		_ = json.NewEncoder(w).Encode(deepSeekResponse{Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{{Message: struct {
			Content string `json:"content"`
		}{Content: `{"reply":"Buka Lapor.","topic":"REPORTING"}`}}}})
	}))
	defer server.Close()

	client := NewDeepSeek("test-key", "deepseek/deepseek-v4-1-flash", t.TempDir())
	if client.endpoint != "https://ai.sinarcloud.com/v1/chat/completions" {
		t.Fatalf("unexpected provider endpoint: %s", client.endpoint)
	}
	client.endpoint = server.URL + "/chat/completions"
	client.client = server.Client()
	if !client.Enabled() {
		t.Fatal("client should be enabled")
	}
	got, err := client.Reply(context.Background(), "Bagaimana cara melapor?", nil, helpapp.SearchContext{})
	if err != nil || got.Reply != "Buka Lapor." || got.Topic != "REPORTING" {
		t.Fatalf("unexpected response %#v, err=%v", got, err)
	}
}
