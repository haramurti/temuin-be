package postgres

import (
	"github.com/google/uuid"
	"github.com/temuin/backend/internal/domain/pickup"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func sp(s string) *string { return &s }

// SeedPickupPoints upserts one Pos Satpam per UB faculty (idempotent by code).
// These double as the "Lokasi" options in the Lapor form and as pos penitipan.
// Coordinates are approximate — adjust with campus security before operational use.
// Codes not in this list (older seeds) are deactivated.
func SeedPickupPoints(db *gorm.DB) error {
	hours := sp("Senin–Jumat 07.00–21.00")
	type row struct {
		code, faculty string
		lat, lng      float64
	}
	rows := []row{
		{"SATPAM-FH", "Fakultas Hukum", -7.95213, 112.61178},
		{"SATPAM-FEB", "Fakultas Ekonomi dan Bisnis", -7.95252, 112.61066},
		{"SATPAM-FIA", "Fakultas Ilmu Administrasi", -7.95462, 112.61128},
		{"SATPAM-FP", "Fakultas Pertanian", -7.95064, 112.61560},
		{"SATPAM-FAPET", "Fakultas Peternakan", -7.95017, 112.61685},
		{"SATPAM-FT", "Fakultas Teknik", -7.94965, 112.61531},
		{"SATPAM-FK", "Fakultas Kedokteran", -7.95580, 112.61510},
		{"SATPAM-FPIK", "Fakultas Perikanan dan Ilmu Kelautan", -7.95300, 112.61770},
		{"SATPAM-FMIPA", "Fakultas Matematika dan Ilmu Pengetahuan Alam", -7.95485, 112.61400},
		{"SATPAM-FTP", "Fakultas Teknologi Pertanian", -7.95210, 112.61720},
		{"SATPAM-FISIP", "Fakultas Ilmu Sosial dan Ilmu Politik", -7.95632, 112.61272},
		{"SATPAM-FIB", "Fakultas Ilmu Budaya", -7.95561, 112.61375},
		{"SATPAM-FKH", "Fakultas Kedokteran Hewan", -7.95715, 112.61540},
		{"SATPAM-FILKOM", "Fakultas Ilmu Komputer", -7.95384, 112.61450},
		{"SATPAM-FIKES", "Fakultas Ilmu Kesehatan", -7.95750, 112.61420},
		{"SATPAM-FKG", "Fakultas Kedokteran Gigi", -7.95790, 112.61330},
		{"SATPAM-VOKASI", "Fakultas Vokasi", -7.95120, 112.61840},
	}
	points := make([]pickup.PickupPoint, 0, len(rows))
	codes := make([]string, 0, len(rows))
	for _, r := range rows {
		points = append(points, pickup.PickupPoint{
			ID: uuid.New(), Code: r.code, Name: "Pos Satpam " + r.faculty, Building: sp(r.faculty),
			Latitude: r.lat, Longitude: r.lng, OpeningHours: hours, IsActive: true,
		})
		codes = append(codes, r.code)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "building", "is_active"}),
		}).Create(&points).Error; err != nil {
			return err
		}
		return tx.Model(&pickup.PickupPoint{}).Where("code NOT IN ?", codes).Update("is_active", false).Error
	})
}
