// Package matching is the report-recommendation domain service (Fungsi 4).
// It implements the 3-stage rule from PRD: mandatory filter, rule-based
// pre-scoring (top 10), then AI comparison. Output is always a recommendation
// and never changes report/claim status.
package matching

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/temuin/backend/internal/domain/report"
)

type Level string

const (
	LevelHigh   Level = "HIGH"   // Tinggi
	LevelMedium Level = "MEDIUM" // Sedang
	LevelLow    Level = "LOW"    // Rendah
)

func (l Level) Valid() bool { return l == LevelHigh || l == LevelMedium || l == LevelLow }

func (l Level) rank() int {
	switch l {
	case LevelHigh:
		return 3
	case LevelMedium:
		return 2
	}
	return 1
}

const (
	AttrCategory = "category"
	AttrColor    = "color"
	AttrLocation = "location"
	AttrTime     = "time"
	AttrPhoto    = "photo"
	AttrKeywords = "keywords"

	MaxWindow         = 30 * 24 * time.Hour
	MaxAICandidates   = 10
	MinCoreComparable = 3
	Disclaimer        = "Rekomendasi, perlu diverifikasi"
)

// PublicView is the only data allowed to leave the system for AI matching:
// public attributes only — no contact, no claim secrets.
type PublicView struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Category     string    `json:"category"`
	Color        string    `json:"color,omitempty"`
	Description  string    `json:"description"`
	OccurredAt   time.Time `json:"occurred_at"`
	LocationName string    `json:"location_name"`
	LocationArea string    `json:"location_area,omitempty"`
	HasPhoto     bool      `json:"has_photo"`
	PhotoURL     string    `json:"-"`
}

func ToPublicView(r *report.Report) PublicView {
	v := PublicView{
		ID: r.ID.String(), Type: string(r.Type), Title: r.Title, Category: string(r.Category),
		Description: r.Description, OccurredAt: r.OccurredAt, LocationName: r.LocationName,
	}
	if r.Color != nil {
		v.Color = *r.Color
	}
	if r.LocationArea != nil {
		v.LocationArea = *r.LocationArea
	}
	if r.PhotoURL != nil && *r.PhotoURL != "" {
		v.HasPhoto, v.PhotoURL = true, *r.PhotoURL
	}
	return v
}

// AIVerdict is what an AIComparer returns per candidate.
type AIVerdict struct {
	ReportID  string   `json:"report_id"`
	Level     Level    `json:"level"`
	Matched   []string `json:"matched_attributes"`
	Different []string `json:"different_attributes"`
	Reason    string   `json:"reason"`
}

// AIComparer is a port implemented by infrastructure (Gemini).
type AIComparer interface {
	Enabled() bool
	Compare(ctx context.Context, source PublicView, candidates []PublicView) ([]AIVerdict, error)
}

// Result is a scored candidate.
type Result struct {
	Candidate      *report.Report
	Level          Level
	Matched        []string
	Different      []string
	NotComparable  []string
	Reason         string
	PreScore       float64
	TimeDiff       time.Duration
	IncompleteData bool
}

// LostAndFound returns (lost, found) ordering for a pair.
func lostFound(a, b *report.Report) (lost, found *report.Report) {
	if a.Type == report.TypeLost {
		return a, b
	}
	return b, a
}

// Eligible: stage 1 mandatory filter.
func Eligible(src, cand *report.Report) bool {
	if cand.ID == src.ID || cand.Type != src.Type.Opposite() || cand.Status != report.StatusActive ||
		cand.UserID == src.UserID || cand.Category != src.Category {
		return false
	}
	if cand.Type == report.TypeFound && cand.Custody != nil && *cand.Custody == report.CustodyUnavailable {
		return false
	}
	lost, found := lostFound(src, cand)
	diff := found.OccurredAt.Sub(lost.OccurredAt)
	return diff >= 0 && diff <= MaxWindow
}

// TimeWindow returns the occurred_at range candidates must fall into.
func TimeWindow(src *report.Report) (from, to time.Time) {
	if src.Type == report.TypeLost {
		return src.OccurredAt, src.OccurredAt.Add(MaxWindow)
	}
	return src.OccurredAt.Add(-MaxWindow), src.OccurredAt
}

func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// PreScore: stage 2 rule-based scoring.
func PreScore(src, cand *report.Report) Result {
	res := Result{Candidate: cand, Matched: []string{AttrCategory}}
	score := 2.0 // same category (guaranteed by filter)
	core := 1    // category comparable

	// color
	if a, b := strPtr(src.Color), strPtr(cand.Color); a != "" && b != "" {
		core++
		if strings.EqualFold(a, b) {
			score += 3
			res.Matched = append(res.Matched, AttrColor)
		} else {
			res.Different = append(res.Different, AttrColor)
		}
	} else {
		res.NotComparable = append(res.NotComparable, AttrColor)
	}

	// location
	core++
	areaA, areaB := strPtr(src.LocationArea), strPtr(cand.LocationArea)
	switch {
	case areaA != "" && areaB != "" && strings.EqualFold(areaA, areaB):
		score += 2
		res.Matched = append(res.Matched, AttrLocation)
	case jaccard(tokens(src.LocationName), tokens(cand.LocationName)) >= 0.3:
		score += 1
		res.Matched = append(res.Matched, AttrLocation)
	default:
		res.Different = append(res.Different, AttrLocation)
	}

	// time
	core++
	lost, found := lostFound(src, cand)
	res.TimeDiff = found.OccurredAt.Sub(lost.OccurredAt)
	switch {
	case res.TimeDiff <= 24*time.Hour:
		score += 2
		res.Matched = append(res.Matched, AttrTime)
	case res.TimeDiff <= 72*time.Hour:
		score += 1
		res.Matched = append(res.Matched, AttrTime)
	default:
		res.Different = append(res.Different, AttrTime)
	}

	// photo: comparable only when both have one (actual comparison is AI's job)
	if strPtr(src.PhotoURL) != "" && strPtr(cand.PhotoURL) != "" {
		core++
	} else {
		res.NotComparable = append(res.NotComparable, AttrPhoto)
	}

	// keywords
	j := jaccard(tokens(src.Title+" "+src.Description), tokens(cand.Title+" "+cand.Description))
	score += 3 * j
	if j >= 0.2 {
		res.Matched = append(res.Matched, AttrKeywords)
	}

	res.PreScore = score
	res.IncompleteData = core < MinCoreComparable
	return res
}

// RuleLevel is the fallback label when AI is not available.
func RuleLevel(score float64) Level {
	switch {
	case score >= 8:
		return LevelHigh
	case score >= 5:
		return LevelMedium
	}
	return LevelLow
}

// Cap enforces the incomplete-data rule (max MEDIUM).
func Cap(r *Result) {
	if r.IncompleteData && r.Level == LevelHigh {
		r.Level = LevelMedium
	}
}

// Sort: label, #matched, pre-score, smaller time diff, newer report.
func Sort(rs []Result) {
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		if a.Level.rank() != b.Level.rank() {
			return a.Level.rank() > b.Level.rank()
		}
		if len(a.Matched) != len(b.Matched) {
			return len(a.Matched) > len(b.Matched)
		}
		if a.PreScore != b.PreScore {
			return a.PreScore > b.PreScore
		}
		if a.TimeDiff != b.TimeDiff {
			return a.TimeDiff < b.TimeDiff
		}
		return a.Candidate.CreatedAt.After(b.Candidate.CreatedAt)
	})
}

var stopwords = map[string]bool{"dan": true, "di": true, "yang": true, "the": true, "a": true, "an": true,
	"warna": true, "dengan": true, "ada": true, "ini": true, "itu": true, "ke": true, "dari": true, "of": true}

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(f) > 2 && !stopwords[f] {
			out[f] = true
		}
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}
