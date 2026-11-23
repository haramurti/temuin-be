// Package dto holds response shapes shared by application services.
// Data is split by sensitivity (PRD "Rancangan Keamanan"): public, restricted, secret.
package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/temuin/backend/internal/domain/pickup"
	"github.com/temuin/backend/internal/domain/report"
	"github.com/temuin/backend/internal/domain/user"
)

type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

func NewMeta(page, limit int, total int64) Meta {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	return Meta{Page: page, Limit: limit, Total: total, TotalPages: (total + int64(limit) - 1) / int64(limit)}
}

// UserPublic is safe to show to any logged-in user.
type UserPublic struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Faculty   *string   `json:"faculty"`
	AvatarURL *string   `json:"avatar_url"`
}

// UserPrivate is only returned to the account owner.
type UserPrivate struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     *string   `json:"email"`
	NIM       *string   `json:"nim"`
	Faculty   *string   `json:"faculty"`
	WhatsApp  *string   `json:"whatsapp"`
	Instagram *string   `json:"instagram"`
	AvatarURL *string   `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Contact is restricted data.
type Contact struct {
	UserID       uuid.UUID `json:"user_id"`
	Name         string    `json:"name"`
	WhatsApp     *string   `json:"whatsapp"`
	Instagram    *string   `json:"instagram"`
	WhatsAppLink *string   `json:"whatsapp_link"`
	InstagramURL *string   `json:"instagram_url"`
}

func ToUserPublic(u *user.User) *UserPublic {
	if u == nil {
		return nil
	}
	return &UserPublic{ID: u.ID, Name: u.Name, Faculty: u.Faculty, AvatarURL: u.AvatarURL}
}

func ToUserPrivate(u *user.User) UserPrivate {
	return UserPrivate{ID: u.ID, Name: u.Name, Email: u.Email, NIM: u.NIM, Faculty: u.Faculty,
		WhatsApp: u.WhatsApp, Instagram: u.Instagram, AvatarURL: u.AvatarURL, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt}
}

func ToContact(u *user.User) Contact {
	c := Contact{UserID: u.ID, Name: u.Name, WhatsApp: u.WhatsApp, Instagram: u.Instagram}
	if u.WhatsApp != nil && *u.WhatsApp != "" {
		l := "https://wa.me/" + normalizeWA(*u.WhatsApp)
		c.WhatsAppLink = &l
	}
	if u.Instagram != nil && *u.Instagram != "" {
		l := "https://instagram.com/" + trimAt(*u.Instagram)
		c.InstagramURL = &l
	}
	return c
}

func normalizeWA(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}
	str := string(out)
	if len(str) > 0 && str[0] == '0' {
		str = "62" + str[1:]
	}
	return str
}

func trimAt(s string) string {
	if len(s) > 0 && s[0] == '@' {
		return s[1:]
	}
	return s
}

type PickupPoint struct {
	ID           uuid.UUID `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	Building     *string   `json:"building"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	OpeningHours *string   `json:"opening_hours"`
	IsActive     bool      `json:"is_active"`
}

func ToPickupPoint(p *pickup.PickupPoint) *PickupPoint {
	if p == nil {
		return nil
	}
	return &PickupPoint{ID: p.ID, Code: p.Code, Name: p.Name, Description: p.Description, Building: p.Building,
		Latitude: p.Latitude, Longitude: p.Longitude, OpeningHours: p.OpeningHours, IsActive: p.IsActive}
}

// Report is the public representation of a report. Never contains claim secrets or contact.
type Report struct {
	ID              uuid.UUID       `json:"id"`
	Type            report.Type     `json:"type"`
	Title           string          `json:"title"`
	Category        report.Category `json:"category"`
	Color           *string         `json:"color"`
	Description     string          `json:"description"`
	OccurredAt      time.Time       `json:"occurred_at"`
	LocationName    string          `json:"location_name"`
	LocationArea    *string         `json:"location_area"`
	LocationPointID *uuid.UUID      `json:"location_point_id"`
	Latitude        *float64        `json:"latitude"`
	Longitude       *float64        `json:"longitude"`
	PhotoURL        *string         `json:"photo_url"`
	Status          report.Status   `json:"status"`
	Custody         *report.Custody `json:"custody"`
	PickupPoint     *PickupPoint    `json:"pickup_point"`
	DepositedAt     *time.Time      `json:"deposited_at"`
	CompletedAt     *time.Time      `json:"completed_at"`
	Owner           *UserPublic     `json:"owner"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Viewer          *ReportViewer   `json:"viewer,omitempty"`
}

// ReportViewer holds per-viewer flags so the client can render the right buttons.
type ReportViewer struct {
	IsOwner       bool    `json:"is_owner"`
	CanClaim      bool    `json:"can_claim"`
	CanContact    bool    `json:"can_contact"`
	MyClaimID     *string `json:"my_claim_id"`
	MyClaimStatus *string `json:"my_claim_status"`
}

func ToReport(r *report.Report, owner *user.User, pp *pickup.PickupPoint) Report {
	return Report{
		ID: r.ID, Type: r.Type, Title: r.Title, Category: r.Category, Color: r.Color, Description: r.Description,
		OccurredAt: r.OccurredAt, LocationName: r.LocationName, LocationArea: r.LocationArea, LocationPointID: r.LocationPointID,
		Latitude: r.Latitude, Longitude: r.Longitude, PhotoURL: r.PhotoURL, Status: r.Status, Custody: r.Custody,
		PickupPoint: ToPickupPoint(pp), DepositedAt: r.DepositedAt, CompletedAt: r.CompletedAt,
		Owner: ToUserPublic(owner), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
