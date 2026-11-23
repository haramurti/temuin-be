// Package pickup implements PickupAvailabilityService (Fungsi 2):
// aggregateAvailability and buildPickupSummary.
package pickup

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/temuin/backend/internal/application/dto"
	reportapp "github.com/temuin/backend/internal/application/report"
	domain "github.com/temuin/backend/internal/domain/pickup"
	"github.com/temuin/backend/internal/domain/report"
	"github.com/temuin/backend/internal/shared/apperror"
)

type Marker struct {
	PickupPoint    dto.PickupPoint `json:"pickup_point"`
	AvailableCount int64           `json:"available_count"`
}

type Availability struct {
	Markers    []Marker `json:"markers"`
	TotalItems int64    `json:"total_items"`
	IsEmpty    bool     `json:"is_empty"`
	Message    *string  `json:"message"`
}

type Summary struct {
	PickupPoint    dto.PickupPoint `json:"pickup_point"`
	AvailableCount int64           `json:"available_count"`
	Items          []dto.Report    `json:"items"`
}

type Service struct {
	pickups domain.Repository
	reports report.Repository
	rs      *reportapp.Service
}

func NewService(p domain.Repository, r report.Repository, rs *reportapp.Service) *Service {
	return &Service{pickups: p, reports: r, rs: rs}
}

// List is the dropdown source: developer-configured active points.
func (s *Service) List(ctx context.Context) ([]dto.PickupPoint, error) {
	ps, err := s.pickups.List(ctx, true)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]dto.PickupPoint, 0, len(ps))
	for i := range ps {
		out = append(out, *dto.ToPickupPoint(&ps[i]))
	}
	return out, nil
}

// Availability: only points with >0 confirmed-deposited items get a marker.
func (s *Service) Availability(ctx context.Context) (*Availability, error) {
	counts, err := s.reports.CountDepositedByPickupPoint(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ps, err := s.pickups.List(ctx, true)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := &Availability{Markers: []Marker{}}
	for i := range ps {
		if n := counts[ps[i].ID]; n > 0 {
			out.Markers = append(out.Markers, Marker{PickupPoint: *dto.ToPickupPoint(&ps[i]), AvailableCount: n})
			out.TotalItems += n
		}
	}
	if out.TotalItems == 0 {
		out.IsEmpty = true
		msg := "Belum ada barang yang dititipkan di pos saat ini"
		out.Message = &msg
	}
	return out, nil
}

// Summary builds the bottom sheet for one point.
func (s *Service) Summary(ctx context.Context, id uuid.UUID, page, limit int) (*Summary, dto.Meta, error) {
	p, err := s.pickups.FindByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, dto.Meta{}, apperror.NotFound("PICKUP_POINT_NOT_FOUND", "Pos tidak ditemukan")
	}
	if err != nil {
		return nil, dto.Meta{}, apperror.Internal(err)
	}
	t, st, cu := report.TypeFound, report.StatusActive, report.CustodyDeposited
	rs, total, err := s.reports.List(ctx, report.Filter{Type: &t, Status: &st, Custody: &cu, PickupPointID: &id, Page: page, Limit: limit})
	if err != nil {
		return nil, dto.Meta{}, apperror.Internal(err)
	}
	items, err := s.rs.Hydrate(ctx, rs)
	if err != nil {
		return nil, dto.Meta{}, err
	}
	return &Summary{PickupPoint: *dto.ToPickupPoint(p), AvailableCount: total, Items: items}, dto.NewMeta(page, limit, total), nil
}
