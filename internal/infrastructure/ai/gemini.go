// Package ai implements matching.AIComparer using the Gemini REST API.
// The API key stays server-side (PRD: kunci tidak disimpan di APK).
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/temuin/backend/internal/domain/matching"
)

type Gemini struct {
	apiKey string
	model  string
	client *http.Client
}

func NewGemini(apiKey, model string) *Gemini {
	return &Gemini{apiKey: apiKey, model: model, client: &http.Client{Timeout: 25 * time.Second}}
}

func (g *Gemini) Enabled() bool { return g.apiKey != "" }

const prompt = `Kamu membantu aplikasi lost & found kampus "Temuin".
Bandingkan LAPORAN SUMBER dengan setiap KANDIDAT (jenis laporan berlawanan).
Gunakan HANYA atribut publik yang diberikan. Atribut kosong = tidak dapat dibandingkan (jangan hitung cocok/beda).
Untuk setiap kandidat kembalikan:
- report_id: id kandidat PERSIS seperti input
- level: salah satu "HIGH" | "MEDIUM" | "LOW"
- matched_attributes: subset dari ["category","color","location","time","photo","keywords"]
- different_attributes: subset dari daftar yang sama
- reason: 1 kalimat Bahasa Indonesia, sebutkan kesamaan dan perbedaan yang perlu dicek.
Ini hanya rekomendasi, BUKAN bukti kepemilikan.
Kembalikan JSON: {"results":[...]}.`

type part struct {
	Text string `json:"text,omitempty"`
}
type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}
type genReq struct {
	Contents         []content      `json:"contents"`
	GenerationConfig map[string]any `json:"generationConfig"`
}
type genResp struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
}

func (g *Gemini) Compare(ctx context.Context, src matching.PublicView, cands []matching.PublicView) ([]matching.AIVerdict, error) {
	if !g.Enabled() {
		return nil, errors.New("gemini disabled")
	}
	payload, _ := json.Marshal(map[string]any{"source": src, "candidates": cands})
	body, _ := json.Marshal(genReq{
		Contents: []content{{Role: "user", Parts: []part{{Text: prompt}, {Text: string(payload)}}}},
		GenerationConfig: map[string]any{
			"responseMimeType": "application/json",
			"temperature":      0.2,
		},
	})
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", g.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini http %d", resp.StatusCode)
	}
	var gr genResp
	if err := json.Unmarshal(raw, &gr); err != nil || len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("gemini: empty or malformed response")
	}
	var out struct {
		Results []matching.AIVerdict `json:"results"`
	}
	if err := json.Unmarshal([]byte(gr.Candidates[0].Content.Parts[0].Text), &out); err != nil {
		return nil, fmt.Errorf("gemini: invalid json: %w", err)
	}
	return out.Results, nil
}
