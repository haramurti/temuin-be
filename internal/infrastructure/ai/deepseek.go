package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const sinarCloudChatEndpoint = "https://ai.sinarcloud.com/v1/chat/completions"

type DeepSeek struct {
	apiKey    string
	model     string
	endpoint  string
	uploadDir string
	client    *http.Client
}

func NewDeepSeek(apiKey, model, uploadDir string) *DeepSeek {
	return &DeepSeek{
		apiKey: strings.TrimSpace(apiKey), model: model, endpoint: sinarCloudChatEndpoint,
		uploadDir: uploadDir, client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (d *DeepSeek) Enabled() bool { return d.apiKey != "" }

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type deepSeekContentPart struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	ImageURL *deepSeekImageURL `json:"image_url,omitempty"`
}

type deepSeekImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type deepSeekRequest struct {
	Model          string            `json:"model"`
	Messages       []deepSeekMessage `json:"messages"`
	Stream         bool              `json:"stream"`
	Thinking       map[string]string `json:"thinking"`
	ResponseFormat map[string]string `json:"response_format"`
	MaxTokens      int               `json:"max_tokens"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (d *DeepSeek) complete(ctx context.Context, messages []deepSeekMessage, maxTokens int) (string, error) {
	if !d.Enabled() {
		return "", fmt.Errorf("DeepSeek is not configured")
	}
	body, err := json.Marshal(deepSeekRequest{
		Model: d.model, Messages: messages, Stream: false,
		Thinking:       map[string]string{"type": "disabled"},
		ResponseFormat: map[string]string{"type": "json_object"}, MaxTokens: maxTokens,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("DeepSeek returned HTTP %d", resp.StatusCode)
	}
	var result deepSeekResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil || len(result.Choices) == 0 {
		return "", fmt.Errorf("DeepSeek returned an invalid response")
	}
	return result.Choices[0].Message.Content, nil
}

func decodeModelJSON(content string, target any) error {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		if newline := strings.IndexByte(content, '\n'); newline >= 0 {
			content = content[newline+1:]
		}
		content = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(content), "```"))
	}
	start, end := strings.IndexByte(content, '{'), strings.LastIndexByte(content, '}')
	if start < 0 || end < start {
		return fmt.Errorf("JSON object not found")
	}
	return json.Unmarshal([]byte(content[start:end+1]), target)
}
