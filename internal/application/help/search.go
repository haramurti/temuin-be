package help

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/temuin/backend/internal/application/dto"
	reportapp "github.com/temuin/backend/internal/application/report"
	reportdomain "github.com/temuin/backend/internal/domain/report"
)

const (
	maxSearchTerms   = 5
	maxSearchReports = 4
	searchPageLimit  = 25
)

type ReportLister interface {
	List(context.Context, reportapp.ListInput, *uuid.UUID) ([]dto.Report, dto.Meta, error)
}

type reportSearcher struct{ reports ReportLister }

func NewReportSearcher(reports ReportLister) ReportSearcher {
	return &reportSearcher{reports: reports}
}

func (s *reportSearcher) Search(ctx context.Context, message string, history []Turn) (SearchContext, error) {
	spec := analyzeReportQuery(message)
	if !spec.lookup && isLikelyFollowUp(message) {
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role != "user" {
				continue
			}
			previous := analyzeReportQuery(history[i].Content)
			if previous.lookup {
				spec = previous
				break
			}
		}
	}
	if !spec.lookup {
		return SearchContext{}, nil
	}

	result := SearchContext{Searched: true, Available: true}
	if s == nil || s.reports == nil {
		result.Available = false
		return result, nil
	}
	queries := buildReportQueries(spec)
	if len(queries) == 0 {
		return result, nil
	}

	collected := make(map[uuid.UUID]dto.Report)
	for _, query := range queries {
		reports, _, err := s.reports.List(ctx, query, nil)
		if err != nil {
			return result, err
		}
		for _, item := range reports {
			if item.Status != reportdomain.StatusActive {
				continue
			}
			if item.Type == reportdomain.TypeFound && item.Custody != nil && *item.Custody == reportdomain.CustodyUnavailable {
				continue
			}
			if !reportMatchesColor(spec.colors, item) {
				continue
			}
			collected[item.ID] = item
		}
	}

	type ranked struct {
		item  dto.Report
		score int
	}
	rankedReports := make([]ranked, 0, len(collected))
	for _, item := range collected {
		haystack := normalizeSearchText(strings.Join([]string{
			item.Title,
			string(item.Category),
			stringValue(item.Color),
			item.Description,
			item.LocationName,
			stringValue(item.LocationArea),
		}, " "))
		score := 0
		if spec.Category != "" && string(item.Category) == spec.Category {
			score += 4
		}
		if spec.Type != "" && string(item.Type) == spec.Type {
			score += 2
		}
		for _, term := range spec.terms {
			if strings.Contains(haystack, term) {
				score++
			}
		}
		rankedReports = append(rankedReports, ranked{item: item, score: score})
	}
	sort.SliceStable(rankedReports, func(i, j int) bool {
		if rankedReports[i].score != rankedReports[j].score {
			return rankedReports[i].score > rankedReports[j].score
		}
		left, right := rankedReports[i].item, rankedReports[j].item
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.After(right.CreatedAt)
		}
		return left.ID.String() < right.ID.String()
	})

	if len(rankedReports) > maxSearchReports {
		rankedReports = rankedReports[:maxSearchReports]
	}
	result.Reports = make([]ReportReference, 0, len(rankedReports))
	for _, candidate := range rankedReports {
		item := candidate.item
		result.Reports = append(result.Reports, ReportReference{
			ID:           item.ID.String(),
			Type:         string(item.Type),
			Title:        item.Title,
			Category:     string(item.Category),
			Color:        stringValue(item.Color),
			LocationName: item.LocationName,
			LocationArea: stringValue(item.LocationArea),
			PhotoURL:     stringValue(item.PhotoURL),
			OccurredAt:   item.OccurredAt,
		})
	}
	return result, nil
}

type reportQuerySpec struct {
	lookup   bool
	Type     string
	Category string
	terms    []string
	colors   []string
}

func analyzeReportQuery(message string) reportQuerySpec {
	text := normalizeSearchText(message)
	words := strings.Fields(text)
	if containsAnyPhrase(text, "cara", "bagaimana cara", "gimana cara") &&
		containsAnyPhrase(text, "lapor", "melapor", "melaporkan", "laporan", "klaim", "cari barang", "unggah") {
		return reportQuerySpec{}
	}
	category := categoryFromText(text)
	reportType := ""
	if containsAnyPhrase(text, "menemukan", "ditemukan", "ditemuin", "temuan", "ketemu", "nemu") {
		reportType = string(reportdomain.TypeFound)
	} else if containsAnyPhrase(text, "kehilangan", "hilang") {
		reportType = string(reportdomain.TypeLost)
	}

	stopWords := map[string]bool{
		"ada": true, "adakah": true, "apa": true, "apakah": true, "bisa": true, "buat": true,
		"bagaimana": true, "gimana": true, "cara": true, "cari": true, "carikan": true,
		"mencari": true, "mencarikan": true, "nyari": true, "nyariin": true, "kamu": true,
		"cek": true, "dengan": true, "di": true, "dimana": true, "hilang": true, "kehilangan": true,
		"ini": true, "itu": true, "saya": true, "aku": true, "kami": true, "orang": true, "kah": true, "dong": true, "boleh": true, "minta": true,
		"yang": true, "menemukan": true, "ditemukan": true, "ditemuin": true, "temuan": true,
		"ketemu": true, "nemu": true, "barang": true, "item": true, "laporan": true, "lapor": true,
		"tolong": true, "bantu": true, "milik": true, "punya": true, "pernah": true, "sudah": true,
		"belum": true, "untuk": true, "dan": true, "atau": true, "ke": true, "dari": true,
		"pada": true, "sekitar": true, "terakhir": true, "warna": true, "berwarna": true,
		"warnanya": true, "enggak": true,
		"gak": true, "nggak": true, "ga": true, "kapan": true, "mana": true, "lokasi": true,
		"lokasinya": true, "tadi": true, "masih": true, "lihat": true, "terdapat": true,
		"ditemukannya": true, "temuin": true,
	}
	for _, alias := range categoryAliases {
		if category == alias.category {
			for _, part := range strings.Fields(alias.text) {
				stopWords[part] = true
			}
		}
	}
	colors := searchColors(text)
	terms := make([]string, 0, len(words))
	seen := map[string]bool{}
	for _, color := range colors {
		for _, alias := range colorAliases[color] {
			for _, word := range strings.Fields(alias) {
				stopWords[word] = true
			}
		}
	}
	for _, word := range words {
		if len([]rune(word)) < 3 || stopWords[word] || seen[word] {
			continue
		}
		seen[word] = true
		terms = append(terms, word)
	}
	if len(terms) > maxSearchTerms {
		terms = terms[:maxSearchTerms]
	}
	lookup := category != "" || reportType != "" || (len(terms) > 0 && containsAnyPhrase(text,
		"ada", "cari", "carikan", "mencari", "mencarikan", "nyari", "nyariin", "cek", "lihat",
		"menemukan", "ditemukan", "temuan", "laporan", "hilang", "kehilangan"))
	return reportQuerySpec{lookup: lookup, Type: reportType, Category: category, terms: terms, colors: colors}
}

type categoryAlias struct{ text, category string }

var categoryAliases = []categoryAlias{
	{text: "kartu mahasiswa", category: "ID_CARD"}, {text: "kartu identitas", category: "ID_CARD"},
	{text: "alat tulis", category: "STATIONERY"}, {text: "jam tangan", category: "ACCESSORIES"},
	{text: "card holder", category: "WALLET"}, {text: "flashdisk", category: "ELECTRONICS"},
	{text: "kacamata", category: "ACCESSORIES"}, {text: "headphone", category: "ELECTRONICS"},
	{text: "earphone", category: "ELECTRONICS"}, {text: "kalkulator", category: "ELECTRONICS"},
	{text: "charger", category: "ELECTRONICS"}, {text: "laptop", category: "ELECTRONICS"},
	{text: "mouse", category: "ELECTRONICS"}, {text: "tablet", category: "ELECTRONICS"},
	{text: "termos", category: "BOTTLE"}, {text: "thermos", category: "BOTTLE"},
	{text: "tumbler", category: "BOTTLE"}, {text: "dompet", category: "WALLET"},
	{text: "botol", category: "BOTTLE"}, {text: "ktm", category: "ID_CARD"},
	{text: "kunci", category: "KEYS"}, {text: "ransel", category: "BAG"},
	{text: "backpack", category: "BAG"}, {text: "jaket", category: "CLOTHING"},
	{text: "sweater", category: "CLOTHING"}, {text: "hoodie", category: "CLOTHING"},
	{text: "baju", category: "CLOTHING"}, {text: "gelang", category: "ACCESSORIES"},
	{text: "cincin", category: "ACCESSORIES"},
	{text: "buku", category: "BOOKS"}, {text: "novel", category: "BOOKS"},
	{text: "pulpen", category: "STATIONERY"}, {text: "pensil", category: "STATIONERY"},
	{text: "penghapus", category: "STATIONERY"}, {text: "tas", category: "BAG"},
}

func categoryFromText(text string) string {
	for _, alias := range categoryAliases {
		if containsAnyPhrase(text, alias.text) {
			return alias.category
		}
	}
	return ""
}

func buildReportQueries(spec reportQuerySpec) []reportapp.ListInput {
	base := reportapp.ListInput{
		Type: spec.Type, Category: spec.Category, Status: string(reportdomain.StatusActive),
		Page: 1, Limit: searchPageLimit,
	}
	var terms []string
	for _, color := range spec.colors {
		terms = append(terms, colorAliases[color]...)
	}
	terms = append(terms, spec.terms...)
	if len(terms) == 0 {
		return []reportapp.ListInput{base}
	}
	if len(terms) > maxSearchTerms {
		terms = terms[:maxSearchTerms]
	}
	queries := make([]reportapp.ListInput, 0, len(terms))
	for _, term := range terms {
		query := base
		query.Query = term
		queries = append(queries, query)
	}
	return queries
}

var colorAliases = map[string][]string{
	"merah":   {"merah", "red"},
	"hijau":   {"hijau", "green"},
	"biru":    {"biru", "navy", "blue"},
	"hitam":   {"hitam", "black"},
	"putih":   {"putih", "white"},
	"cokelat": {"cokelat", "coklat", "brown"},
	"oranye":  {"oranye", "jingga", "orange"},
	"kuning":  {"kuning", "yellow"},
	"ungu":    {"ungu", "purple"},
	"abu":     {"abu", "gray", "grey"},
}

func searchColors(text string) []string {
	var colors []string
	for color, aliases := range colorAliases {
		if containsAnyPhrase(text, aliases...) {
			colors = append(colors, color)
		}
	}
	sort.Strings(colors)
	return colors
}

func reportMatchesColor(colors []string, item dto.Report) bool {
	if len(colors) == 0 {
		return true
	}
	text := normalizeSearchText(stringValue(item.Color))
	if len(searchColors(text)) == 0 {
		text = normalizeSearchText(item.Title + " " + item.Description)
	}
	for _, color := range colors {
		if containsAnyPhrase(text, colorAliases[color]...) {
			return true
		}
	}
	return false
}

func isLikelyFollowUp(message string) bool {
	text := normalizeSearchText(message)
	return containsAnyPhrase(text, "dimana", "lokasi", "lokasinya", "alamat", "kapan", "foto", "detail", "yang mana", "barang itu", "laporan itu", "masih ada")
}

func containsAnyPhrase(text string, phrases ...string) bool {
	text = " " + text + " "
	for _, phrase := range phrases {
		if strings.Contains(text, " "+normalizeSearchText(phrase)+" ") {
			return true
		}
	}
	return false
}

func normalizeSearchText(text string) string {
	var normalized strings.Builder
	for _, char := range strings.ToLower(text) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			normalized.WriteRune(char)
		} else {
			normalized.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(normalized.String()), " ")
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
