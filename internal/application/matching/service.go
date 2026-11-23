// Package matching implements ReportMatchingService (Fungsi 4):
// selectCandidates, requestMatches, validateMatchResult.
package matching

import (
	"context"
	"errors"
	"log"
	"sort"

	"github.com/google/uuid"
	"github.com/temuin/backend/internal/application/dto"
	reportapp "github.com/temuin/backend/internal/application/report"
	domain "github.com/temuin/backend/internal/domain/matching"
	"github.com/temuin/backend/internal/domain/report"
	"github.com/temuin/backend/internal/shared/apperror"
)

const (
	SourceAI   = "AI"
	SourceRule = "RULE_BASED"
)

type Item struct {
	Report              dto.Report   `json:"report"`
	Level               domain.Level `json:"level"`
	MatchedAttributes   []string     `json:"matched_attributes"`
	DifferentAttributes []string     `json:"different_attributes"`
	NotComparable       []string     `json:"not_comparable_attributes"`
	Reason              string       `json:"reason"`
	PreScore            float64      `json:"pre_score"`
	IncompleteData      bool         `json:"incomplete_data"`
}

type Result struct {
	SourceReportID  uuid.UUID `json:"source_report_id"`
	Source          string    `json:"source"`
	Label           string    `json:"label"`
	Notices         []string  `json:"notices"`
	CandidatesFound int       `json:"candidates_found"`
	Items           []Item    `json:"items"`
}

type Service struct {
	reports report.Repository
	ai      domain.AIComparer
	rs      *reportapp.Service
}

func NewService(r report.Repository, ai domain.AIComparer, rs *reportapp.Service) *Service {
	return &Service{reports: r, ai: ai, rs: rs}
}

func (s *Service) Recommend(ctx context.Context, uid, reportID uuid.UUID) (*Result, error) {
	src, err := s.reports.FindByID(ctx, reportID)
	if errors.Is(err, report.ErrNotFound) {
		return nil, apperror.NotFound("REPORT_NOT_FOUND", "Laporan tidak ditemukan")
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if src.Status != report.StatusActive {
		return nil, apperror.Unprocessable("REPORT_NOT_ACTIVE", "Rekomendasi hanya untuk laporan aktif")
	}
	res := &Result{SourceReportID: src.ID, Source: SourceRule, Label: domain.Disclaimer, Notices: []string{}, Items: []Item{}}

	// Stage 1 — selectCandidates (mandatory filter).
	from, to := domain.TimeWindow(src)
	raw, err := s.reports.FindCandidates(ctx, report.CandidateQuery{
		Type: src.Type.Opposite(), Category: src.Category, ExcludeUserID: src.UserID, From: from, To: to,
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var scored []domain.Result
	for i := range raw {
		if domain.Eligible(src, &raw[i]) {
			scored = append(scored, domain.PreScore(src, &raw[i]))
		}
	}
	res.CandidatesFound = len(scored)
	if len(scored) == 0 {
		res.Notices = append(res.Notices, "Belum ada kandidat yang cocok. Gunakan pencarian manual.")
		return res, nil
	}

	// Stage 2 — top-N by pre-score.
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].PreScore > scored[j].PreScore })
	if len(scored) > domain.MaxAICandidates {
		scored = scored[:domain.MaxAICandidates]
	}

	// Stage 3 — requestMatches (AI) with validateMatchResult, else rule-based fallback.
	usedAI := false
	if s.ai != nil && s.ai.Enabled() {
		views := make([]domain.PublicView, len(scored))
		for i := range scored {
			views[i] = domain.ToPublicView(scored[i].Candidate)
		}
		verdicts, aerr := s.ai.Compare(ctx, domain.ToPublicView(src), views)
		if aerr == nil {
			usedAI = s.apply(scored, verdicts)
		} else {
			log.Printf("matching: AI failed: %v", aerr)
		}
		if !usedAI {
			res.Notices = append(res.Notices, "Layanan AI sedang tidak tersedia; hasil memakai penilaian berbasis aturan.")
		}
	}
	if usedAI {
		res.Source = SourceAI
	} else {
		for i := range scored {
			scored[i].Level = domain.RuleLevel(scored[i].PreScore)
			scored[i].Reason = "Penilaian berbasis aturan: kesamaan kategori, warna, lokasi, waktu, dan kata kunci."
		}
	}
	incomplete := false
	for i := range scored {
		domain.Cap(&scored[i])
		incomplete = incomplete || scored[i].IncompleteData
	}
	if incomplete {
		res.Notices = append(res.Notices, "Sebagian data laporan belum lengkap; tingkat kecocokan dibatasi maksimal Sedang.")
	}
	domain.Sort(scored)

	reps := make([]report.Report, len(scored))
	for i := range scored {
		reps[i] = *scored[i].Candidate
	}
	hyd, err := s.rs.Hydrate(ctx, reps)
	if err != nil {
		return nil, err
	}
	for i := range scored {
		res.Items = append(res.Items, Item{
			Report: hyd[i], Level: scored[i].Level, MatchedAttributes: nz(scored[i].Matched),
			DifferentAttributes: nz(scored[i].Different), NotComparable: nz(scored[i].NotComparable),
			Reason: scored[i].Reason, PreScore: round2(scored[i].PreScore), IncompleteData: scored[i].IncompleteData,
		})
	}
	return res, nil
}

var validAttr = map[string]bool{domain.AttrCategory: true, domain.AttrColor: true, domain.AttrLocation: true,
	domain.AttrTime: true, domain.AttrPhoto: true, domain.AttrKeywords: true}

// apply validates AI output: IDs must be from the sent set, labels must be valid.
// Attributes marked not-comparable by rules are never counted as match/diff.
func (s *Service) apply(scored []domain.Result, vs []domain.AIVerdict) bool {
	idx := map[string]int{}
	for i := range scored {
		idx[scored[i].Candidate.ID.String()] = i
	}
	applied := 0
	seen := map[string]bool{}
	for _, v := range vs {
		i, ok := idx[v.ReportID]
		if !ok || seen[v.ReportID] || !v.Level.Valid() {
			continue
		}
		seen[v.ReportID] = true
		nc := map[string]bool{}
		for _, a := range scored[i].NotComparable {
			nc[a] = true
		}
		scored[i].Level = v.Level
		scored[i].Matched = filter(v.Matched, nc)
		scored[i].Different = filter(v.Different, nc)
		scored[i].Reason = v.Reason
		applied++
	}
	if applied == 0 {
		return false
	}
	// candidates the AI skipped fall back to rule level
	for i := range scored {
		if !seen[scored[i].Candidate.ID.String()] {
			scored[i].Level = domain.RuleLevel(scored[i].PreScore)
			scored[i].Reason = "Tidak dinilai AI; memakai penilaian berbasis aturan."
		}
	}
	return true
}

func filter(in []string, nc map[string]bool) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, a := range in {
		if validAttr[a] && !nc[a] && !seen[a] {
			out = append(out, a)
			seen[a] = true
		}
	}
	return out
}

func nz(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
