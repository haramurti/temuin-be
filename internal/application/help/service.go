package help

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxMessageRunes = 500
const MaxHistoryMessages = 4

var ErrInvalidInput = errors.New("pesan bantuan tidak valid")

type Topic struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Answer      string `json:"answer"`
}

type Turn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Reply struct {
	Reply   string            `json:"reply"`
	Topic   string            `json:"topic"`
	Source  string            `json:"source"`
	Reports []ReportReference `json:"reports"`
}

// ReportReference contains only public fields needed to show a report result in chat.
type ReportReference struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Category     string    `json:"category"`
	Color        string    `json:"color,omitempty"`
	LocationName string    `json:"location_name"`
	LocationArea string    `json:"location_area,omitempty"`
	PhotoURL     string    `json:"photo_url,omitempty"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// SearchContext is a bounded lookup of active, public report records.
type SearchContext struct {
	Searched  bool
	Available bool
	Reports   []ReportReference
}

type Responder interface {
	Reply(context.Context, string, []Turn, SearchContext) (Reply, error)
}

type ReportSearcher interface {
	Search(context.Context, string, []Turn) (SearchContext, error)
}

type Service struct {
	responder Responder
	searcher  ReportSearcher
}

func NewService(responder Responder, searcher ReportSearcher) *Service {
	return &Service{responder: responder, searcher: searcher}
}

func (s *Service) Topics() []Topic { return append([]Topic(nil), topics...) }

func (s *Service) Chat(ctx context.Context, message string, history []Turn) (Reply, error) {
	message = strings.TrimSpace(message)
	if message == "" || utf8.RuneCountInString(message) > MaxMessageRunes || len(history) > MaxHistoryMessages {
		return Reply{}, ErrInvalidInput
	}
	for _, turn := range history {
		if (turn.Role != "user" && turn.Role != "assistant") || strings.TrimSpace(turn.Content) == "" || utf8.RuneCountInString(turn.Content) > MaxMessageRunes {
			return Reply{}, ErrInvalidInput
		}
	}
	var searchContext SearchContext
	if s.searcher != nil {
		var err error
		searchContext, err = s.searcher.Search(ctx, message, history)
		if err != nil && searchContext.Searched {
			searchContext.Available = false
		}
	}
	if searchContext.Searched && !searchContext.Available {
		return withReportList(Reply{
			Reply:  "Saya belum bisa memeriksa laporan saat ini. Coba lagi sebentar lagi atau cari langsung dari Beranda.",
			Topic:  "SEARCH",
			Source: "FAQ",
		}), nil
	}
	if searchContext.Searched && len(searchContext.Reports) == 0 {
		return withReportList(reportFallback(searchContext)), nil
	}
	if s.responder != nil {
		reply, err := s.responder.Reply(ctx, message, history, searchContext)
		if err == nil {
			reply.Reply = strings.TrimSpace(reply.Reply)
			reply.Topic = normalizeTopic(reply.Topic)
			if reply.Reply != "" {
				reply.Source = "DEEPSEEK"
				reply.Reports = searchContext.Reports
				if searchContext.Searched {
					reply.Topic = "SEARCH"
				}
				return withReportList(reply), nil
			}
		}
	}
	if searchContext.Searched {
		return withReportList(reportFallback(searchContext)), nil
	}
	return withReportList(fallback(message)), nil
}

func withReportList(reply Reply) Reply {
	if reply.Reports == nil {
		reply.Reports = []ReportReference{}
	}
	return reply
}

func reportFallback(searchContext SearchContext) Reply {
	if len(searchContext.Reports) == 0 {
		return Reply{
			Reply:  "Saya belum menemukan laporan aktif yang cocok dengan ciri itu. Coba kata kunci lain atau lihat daftar laporan di Beranda.",
			Topic:  "SEARCH",
			Source: "DATABASE",
		}
	}
	return Reply{
		Reply:   "Saya menemukan laporan aktif yang mungkin cocok. Periksa foto, lokasi, dan detailnya sebelum menyimpulkan barang tersebut milikmu.",
		Topic:   "SEARCH",
		Source:  "DATABASE",
		Reports: searchContext.Reports,
	}
}

func normalizeTopic(topic string) string {
	topic = strings.ToUpper(strings.TrimSpace(topic))
	for _, item := range topics {
		if item.ID == topic {
			return topic
		}
	}
	return "GENERAL"
}

func fallback(message string) Reply {
	value := strings.ToLower(message)
	topic := "GENERAL"
	switch {
	case strings.Contains(value, "klaim"), strings.Contains(value, "inbox"), strings.Contains(value, "chat penemu"), strings.Contains(value, "persetujuan"):
		topic = "CLAIM_INBOX"
	case strings.Contains(value, "peta"), strings.Contains(value, "titip"), strings.Contains(value, "penitipan"), strings.Contains(value, "pos satpam"):
		topic = "MAP_CUSTODY"
	case strings.Contains(value, "hilang"), strings.Contains(value, "temukan"), strings.Contains(value, "cari barang"), strings.Contains(value, "kecocokan"):
		topic = "SEARCH"
	case strings.Contains(value, "lapor"), strings.Contains(value, "unggah"), strings.Contains(value, "foto"):
		topic = "REPORTING"
	}
	for _, item := range topics {
		if item.ID == topic {
			return Reply{Reply: item.Answer, Topic: topic, Source: "FAQ"}
		}
	}
	return Reply{Reply: "Saya bisa membantu membuat laporan, mencari barang, memakai peta dan penitipan, atau mengajukan klaim. Pilih salah satu topik bantuan untuk langkah-langkahnya.", Topic: "GENERAL", Source: "FAQ"}
}

var topics = []Topic{
	{ID: "REPORTING", Title: "Buat Laporan", Description: "Laporkan barang yang hilang atau ditemukan.", Answer: "Buka Beranda, tekan tombol Lapor, pilih barang hilang atau ditemukan, lalu isi judul, kategori, ciri, lokasi, waktu, dan foto jika ada. Periksa kembali detailnya sebelum mengirim."},
	{ID: "SEARCH", Title: "Cari Barang", Description: "Cari laporan dan periksa rekomendasi kecocokan.", Answer: "Cari laporan dari Beranda memakai kolom pencarian dan filter. Pada laporan barang hilang milikmu, buka Detail lalu tekan Cari Kecocokan. Rekomendasi membantu penyaringan, tetapi ciri khusus tetap perlu diperiksa sebelum mengajukan klaim."},
	{ID: "MAP_CUSTODY", Title: "Peta & Penitipan", Description: "Lihat lokasi dan titik pengambilan di kampus.", Answer: "Buka Peta untuk melihat titik pengambilan yang tersedia. Saat membuat laporan barang temuan, pilih titik pengambilan dari daftar lokasi. Pastikan status penitipan barang sudah sesuai sebelum mengarahkan pemilik mengambilnya."},
	{ID: "CLAIM_INBOX", Title: "Klaim & Inbox", Description: "Ajukan klaim dan pantau pembaruan laporan.", Answer: "Pada Detail barang temuan, ajukan klaim dengan menjelaskan ciri khusus yang tidak terlihat di laporan. Pantau Inbox untuk persetujuan atau penolakan. Jika disetujui, ikuti petunjuk pengambilan yang ditampilkan aplikasi."},
}
