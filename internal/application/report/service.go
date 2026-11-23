package report

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/temuin/backend/internal/application/dto"
	"github.com/temuin/backend/internal/application/events"
	"github.com/temuin/backend/internal/domain/claim"
	"github.com/temuin/backend/internal/domain/pickup"
	domain "github.com/temuin/backend/internal/domain/report"
	"github.com/temuin/backend/internal/domain/user"
	"github.com/temuin/backend/internal/shared/apperror"
	"github.com/temuin/backend/internal/shared/tx"
)

type CreateInput struct {
	Type        domain.Type     `json:"type" validate:"required,oneof=LOST FOUND"`
	Title       string          `json:"title" validate:"required,min=3,max=120"`
	Category    domain.Category `json:"category" validate:"required"`
	Color       *string         `json:"color" validate:"omitempty,max=40"`
	Description string          `json:"description" validate:"required,min=5,max=2000"`
	OccurredAt  time.Time       `json:"occurred_at" validate:"required"`
	// Either pick a pos satpam/fakultas (location_point_id) or type location_name; both may be sent
	// (point = area, name = detail spot, e.g. "Kantin, Meja 12").
	LocationPointID *uuid.UUID      `json:"location_point_id"`
	LocationName    string          `json:"location_name" validate:"omitempty,min=2,max=160"`
	LocationArea    *string         `json:"location_area" validate:"omitempty,max=120"`
	Latitude        *float64        `json:"latitude" validate:"omitempty,latitude"`
	Longitude       *float64        `json:"longitude" validate:"omitempty,longitude"`
	PhotoURL        *string         `json:"photo_url" validate:"omitempty,url,max=512"`
	Custody         *domain.Custody `json:"custody" validate:"omitempty,oneof=HELD_BY_FINDER AWAITING_DROPOFF"`
	PickupPointID   *uuid.UUID      `json:"pickup_point_id"`
}

type UpdateInput struct {
	Title           *string          `json:"title" validate:"omitempty,min=3,max=120"`
	Category        *domain.Category `json:"category"`
	Color           *string          `json:"color" validate:"omitempty,max=40"`
	Description     *string          `json:"description" validate:"omitempty,min=5,max=2000"`
	OccurredAt      *time.Time       `json:"occurred_at"`
	LocationPointID *uuid.UUID       `json:"location_point_id"`
	LocationName    *string          `json:"location_name" validate:"omitempty,min=2,max=160"`
	LocationArea    *string          `json:"location_area" validate:"omitempty,max=120"`
	Latitude        *float64         `json:"latitude" validate:"omitempty,latitude"`
	Longitude       *float64         `json:"longitude" validate:"omitempty,longitude"`
	PhotoURL        *string          `json:"photo_url" validate:"omitempty,url,max=512"`
}

type CustodyInput struct {
	Custody       domain.Custody `json:"custody" validate:"required,oneof=HELD_BY_FINDER AWAITING_DROPOFF UNAVAILABLE"`
	PickupPointID *uuid.UUID     `json:"pickup_point_id"`
}

type ListInput struct {
	Query    string
	Type     string
	Category string
	Status   string
	Page     int
	Limit    int
}

type Service struct {
	tx       tx.Manager
	reports  domain.Repository
	users    user.Repository
	pickups  pickup.Repository
	claims   claim.Repository
	notifier *events.Notifier
	now      func() time.Time
}

func NewService(t tx.Manager, r domain.Repository, u user.Repository, p pickup.Repository, c claim.Repository, n *events.Notifier) *Service {
	return &Service{tx: t, reports: r, users: u, pickups: p, claims: c, notifier: n, now: func() time.Time { return time.Now().UTC() }}
}

var (
	errNotFound  = apperror.NotFound("REPORT_NOT_FOUND", "Laporan tidak ditemukan")
	errNotOwner  = apperror.Forbidden("NOT_REPORT_OWNER", "Hanya pengunggah laporan yang dapat melakukan aksi ini")
	errNotActive = apperror.Unprocessable("REPORT_NOT_ACTIVE", "Laporan sudah tidak aktif")
)

func (s *Service) load(ctx context.Context, id uuid.UUID, lock bool) (*domain.Report, error) {
	var r *domain.Report
	var err error
	if lock {
		r, err = s.reports.FindByIDForUpdate(ctx, id)
	} else {
		r, err = s.reports.FindByID(ctx, id)
	}
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return r, nil
}

func (s *Service) activePickup(ctx context.Context, id *uuid.UUID) (*pickup.PickupPoint, error) {
	if id == nil || *id == uuid.Nil {
		return nil, apperror.Validation(map[string]string{"pickup_point_id": "wajib diisi jika barang dititipkan di pos"})
	}
	p, err := s.pickups.FindByID(ctx, *id)
	if errors.Is(err, pickup.ErrNotFound) || (err == nil && !p.IsActive) {
		return nil, apperror.Validation(map[string]string{"pickup_point_id": "pos tidak ditemukan atau tidak aktif"})
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return p, nil
}

func mapDomainErr(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFoundType):
		return apperror.Unprocessable("FOUND_REPORT_ONLY", "Aksi ini hanya untuk laporan barang ditemukan")
	case errors.Is(err, domain.ErrNotActive):
		return errNotActive
	case errors.Is(err, domain.ErrInvalidCustodyState):
		return apperror.Unprocessable("INVALID_CUSTODY_STATE", "Status penitipan tidak valid untuk aksi ini")
	case errors.Is(err, domain.ErrPickupPointRequired):
		return apperror.Validation(map[string]string{"pickup_point_id": "wajib diisi"})
	}
	return apperror.Internal(err)
}

func (s *Service) Create(ctx context.Context, uid uuid.UUID, in CreateInput) (*dto.Report, error) {
	if !in.Category.Valid() {
		return nil, apperror.Validation(map[string]string{"category": "kategori tidak dikenal"})
	}
	if in.OccurredAt.After(s.now().Add(5 * time.Minute)) {
		return nil, apperror.Validation(map[string]string{"occurred_at": "waktu kejadian tidak boleh di masa depan"})
	}
	r := &domain.Report{
		ID: uuid.New(), UserID: uid, Type: in.Type, Title: strings.TrimSpace(in.Title), Category: in.Category,
		Color: in.Color, Description: strings.TrimSpace(in.Description), OccurredAt: in.OccurredAt.UTC(),
		LocationName: strings.TrimSpace(in.LocationName), LocationArea: in.LocationArea,
		Latitude: in.Latitude, Longitude: in.Longitude, PhotoURL: in.PhotoURL, Status: domain.StatusActive,
	}
	if in.LocationPointID != nil {
		if err := s.applyLocationPoint(ctx, r, *in.LocationPointID); err != nil {
			return nil, err
		}
	} else if r.LocationName == "" {
		return nil, apperror.Validation(map[string]string{"location_point_id": "pilih lokasi atau isi location_name"})
	}
	var pp *pickup.PickupPoint
	if in.Type == domain.TypeLost {
		if in.Custody != nil || in.PickupPointID != nil {
			return nil, apperror.Validation(map[string]string{"custody": "hanya untuk laporan barang ditemukan"})
		}
	} else {
		custody := domain.CustodyHeldByFinder
		if in.Custody != nil {
			custody = *in.Custody
		}
		if custody == domain.CustodyAwaitingDropoff {
			p, err := s.activePickup(ctx, in.PickupPointID)
			if err != nil {
				return nil, err
			}
			pp = p
			if err := r.RequestDeposit(p.ID); err != nil {
				return nil, mapDomainErr(err)
			}
		} else {
			_ = r.SetHeldByFinder()
		}
	}
	if err := s.reports.Create(ctx, r); err != nil {
		return nil, apperror.Internal(err)
	}
	owner, _ := s.users.FindByID(ctx, uid)
	out := dto.ToReport(r, owner, pp)
	out.Viewer = &dto.ReportViewer{IsOwner: true}
	return &out, nil
}

// Get re-reads current status every time (PRD Fungsi 2: status diperiksa ulang saat detail dibuka).
func (s *Service) Get(ctx context.Context, viewer uuid.UUID, id uuid.UUID) (*dto.Report, error) {
	r, err := s.load(ctx, id, false)
	if err != nil {
		return nil, err
	}
	list, err := s.hydrate(ctx, []domain.Report{*r})
	if err != nil {
		return nil, err
	}
	out := list[0]
	v := &dto.ReportViewer{IsOwner: r.IsOwnedBy(viewer)}
	if r.Type == domain.TypeLost {
		v.CanContact = !v.IsOwner && r.Status == domain.StatusActive
	} else if !v.IsOwner {
		cs, err := s.claims.ListByReport(ctx, r.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		open := false
		for i := len(cs) - 1; i >= 0; i-- {
			c := cs[i]
			if c.ClaimantID != viewer {
				continue
			}
			if v.MyClaimID == nil {
				id, st := c.ID.String(), string(c.Status)
				v.MyClaimID, v.MyClaimStatus = &id, &st
			}
			if c.Status == claim.StatusPending || c.Status == claim.StatusApproved {
				open = true
			}
			if c.Status == claim.StatusApproved || c.Status == claim.StatusCompleted {
				v.CanContact = true
			}
		}
		approvedExists, err := s.claims.HasApproved(ctx, r.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		v.CanClaim = r.IsAvailableForClaim() && !open && !approvedExists
	}
	out.Viewer = v
	return &out, nil
}

func (s *Service) List(ctx context.Context, in ListInput, owner *uuid.UUID) ([]dto.Report, dto.Meta, error) {
	f := domain.Filter{Query: in.Query, Page: in.Page, Limit: in.Limit, UserID: owner}
	if in.Type != "" {
		t := domain.Type(strings.ToUpper(in.Type))
		if !t.Valid() {
			return nil, dto.Meta{}, apperror.Validation(map[string]string{"type": "harus LOST atau FOUND"})
		}
		f.Type = &t
	}
	if in.Category != "" {
		c := domain.Category(strings.ToUpper(in.Category))
		if !c.Valid() {
			return nil, dto.Meta{}, apperror.Validation(map[string]string{"category": "kategori tidak dikenal"})
		}
		f.Category = &c
	}
	switch st := strings.ToUpper(in.Status); st {
	case "":
		if owner == nil { // public feed defaults to active only
			a := domain.StatusActive
			f.Status = &a
		}
	case "ALL":
	case string(domain.StatusActive), string(domain.StatusCompleted), string(domain.StatusClosed):
		x := domain.Status(st)
		f.Status = &x
	default:
		return nil, dto.Meta{}, apperror.Validation(map[string]string{"status": "harus ACTIVE, COMPLETED, CLOSED, atau ALL"})
	}
	rs, total, err := s.reports.List(ctx, f)
	if err != nil {
		return nil, dto.Meta{}, apperror.Internal(err)
	}
	out, err := s.hydrate(ctx, rs)
	return out, dto.NewMeta(in.Page, in.Limit, total), err
}

// hydrate batch-loads owners and pickup points.
func (s *Service) hydrate(ctx context.Context, rs []domain.Report) ([]dto.Report, error) {
	uids := make([]uuid.UUID, 0, len(rs))
	pids := []uuid.UUID{}
	for _, r := range rs {
		uids = append(uids, r.UserID)
		if r.PickupPointID != nil {
			pids = append(pids, *r.PickupPointID)
		}
	}
	users, err := s.users.FindByIDs(ctx, uids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	pps, err := s.pickups.FindByIDs(ctx, pids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]dto.Report, 0, len(rs))
	for i := range rs {
		var pp *pickup.PickupPoint
		if rs[i].PickupPointID != nil {
			pp = pps[*rs[i].PickupPointID]
		}
		out = append(out, dto.ToReport(&rs[i], users[rs[i].UserID], pp))
	}
	return out, nil
}

func (s *Service) Hydrate(ctx context.Context, rs []domain.Report) ([]dto.Report, error) {
	return s.hydrate(ctx, rs)
}

func (s *Service) Update(ctx context.Context, uid, id uuid.UUID, in UpdateInput) (*dto.Report, error) {
	r, err := s.load(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if !r.IsOwnedBy(uid) {
		return nil, errNotOwner
	}
	if r.Status != domain.StatusActive {
		return nil, errNotActive
	}
	if in.Title != nil {
		r.Title = strings.TrimSpace(*in.Title)
	}
	if in.Category != nil {
		if !in.Category.Valid() {
			return nil, apperror.Validation(map[string]string{"category": "kategori tidak dikenal"})
		}
		r.Category = *in.Category
	}
	if in.Color != nil {
		r.Color = in.Color
	}
	if in.Description != nil {
		r.Description = strings.TrimSpace(*in.Description)
	}
	if in.OccurredAt != nil {
		if in.OccurredAt.After(s.now().Add(5 * time.Minute)) {
			return nil, apperror.Validation(map[string]string{"occurred_at": "waktu kejadian tidak boleh di masa depan"})
		}
		r.OccurredAt = in.OccurredAt.UTC()
	}
	if in.LocationPointID != nil {
		if err := s.applyLocationPoint(ctx, r, *in.LocationPointID); err != nil {
			return nil, err
		}
	}
	if in.LocationName != nil {
		r.LocationName = strings.TrimSpace(*in.LocationName)
	}
	if in.LocationArea != nil {
		r.LocationArea = in.LocationArea
	}
	if in.Latitude != nil {
		r.Latitude = in.Latitude
	}
	if in.Longitude != nil {
		r.Longitude = in.Longitude
	}
	if in.PhotoURL != nil {
		r.PhotoURL = in.PhotoURL
	}
	if err := s.reports.Update(ctx, r); err != nil {
		return nil, apperror.Internal(err)
	}
	return s.Get(ctx, uid, id)
}

// Delete: only reports without any claims history; otherwise use Close.
func (s *Service) Delete(ctx context.Context, uid, id uuid.UUID) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.load(ctx, id, true)
		if err != nil {
			return err
		}
		if !r.IsOwnedBy(uid) {
			return errNotOwner
		}
		has, err := s.claims.ExistsForReport(ctx, id)
		if err != nil {
			return apperror.Internal(err)
		}
		if has {
			return apperror.Conflict("REPORT_HAS_CLAIMS", "Laporan sudah memiliki riwayat klaim; gunakan tutup laporan")
		}
		if err := s.reports.Delete(ctx, id); err != nil {
			return apperror.Internal(err)
		}
		return nil
	})
}

// Close: owner closes an active report without handover (e.g. lost item found by self).
// Open claims become ITEM_UNAVAILABLE.
func (s *Service) Close(ctx context.Context, uid, id uuid.UUID) (*dto.Report, error) {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.load(ctx, id, true)
		if err != nil {
			return err
		}
		if !r.IsOwnedBy(uid) {
			return errNotOwner
		}
		approved, err := s.claims.HasApproved(ctx, r.ID)
		if err != nil {
			return apperror.Internal(err)
		}
		if approved {
			return apperror.Conflict("HAS_APPROVED_CLAIM", "Ada klaim yang sudah disetujui; selesaikan serah terima melalui klaim")
		}
		if err := r.Close(s.now()); err != nil {
			return mapDomainErr(err)
		}
		if err := s.reports.Update(ctx, r); err != nil {
			return apperror.Internal(err)
		}
		return s.invalidateOpenClaims(ctx, r)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, uid, id)
}

// UpdateCustody: hold / choose-or-change pos / mark unavailable.
func (s *Service) UpdateCustody(ctx context.Context, uid, id uuid.UUID, in CustodyInput) (*dto.Report, error) {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.load(ctx, id, true)
		if err != nil {
			return err
		}
		if !r.IsOwnedBy(uid) {
			return errNotOwner
		}
		switch in.Custody {
		case domain.CustodyHeldByFinder:
			err = r.SetHeldByFinder()
		case domain.CustodyAwaitingDropoff:
			p, perr := s.activePickup(ctx, in.PickupPointID)
			if perr != nil {
				return perr
			}
			err = r.RequestDeposit(p.ID)
		case domain.CustodyUnavailable:
			err = r.MarkUnavailable()
		}
		if err != nil {
			return mapDomainErr(err)
		}
		if err := s.reports.Update(ctx, r); err != nil {
			return apperror.Internal(err)
		}
		if in.Custody == domain.CustodyUnavailable {
			return s.invalidateOpenClaims(ctx, r)
		}
		return s.refreshApprovedPickup(ctx, r)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, uid, id)
}

func (s *Service) ConfirmDeposit(ctx context.Context, uid, id uuid.UUID) (*dto.Report, error) {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.load(ctx, id, true)
		if err != nil {
			return err
		}
		if !r.IsOwnedBy(uid) {
			return errNotOwner
		}
		if err := r.ConfirmDeposit(s.now()); err != nil {
			return mapDomainErr(err)
		}
		if err := s.reports.Update(ctx, r); err != nil {
			return apperror.Internal(err)
		}
		return s.refreshApprovedPickup(ctx, r)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, uid, id)
}

// Contact: LOST → reporter's own contact for "Hubungi Pelapor".
// FOUND → finder contact only for a claimant whose claim is approved/completed.
func (s *Service) Contact(ctx context.Context, viewer, id uuid.UUID) (*dto.Contact, error) {
	r, err := s.load(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if r.IsOwnedBy(viewer) {
		return nil, apperror.Unprocessable("OWN_REPORT", "Ini laporan milikmu sendiri")
	}
	if r.Type == domain.TypeFound {
		ok, err := s.claims.ExistsBetween(ctx, r.ID, viewer, []claim.Status{claim.StatusApproved, claim.StatusCompleted})
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if !ok {
			return nil, apperror.Forbidden("CONTACT_RESTRICTED", "Kontak penemu hanya tersedia setelah klaim disetujui")
		}
	} else if r.Status != domain.StatusActive {
		return nil, errNotActive
	}
	u, err := s.users.FindByID(ctx, r.UserID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	c := dto.ToContact(u)
	return &c, nil
}

func (s *Service) invalidateOpenClaims(ctx context.Context, r *domain.Report) error {
	cs, err := s.claims.ListByReport(ctx, r.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	for i := range cs {
		if cs[i].MarkItemUnavailable() {
			if err := s.claims.Update(ctx, &cs[i]); err != nil {
				return apperror.Internal(err)
			}
			if err := s.notifier.ClaimStatusChanged(ctx, &cs[i], r); err != nil {
				return apperror.Internal(err)
			}
		}
	}
	return nil
}

func (s *Service) refreshApprovedPickup(ctx context.Context, r *domain.Report) error {
	cs, err := s.claims.ListByReport(ctx, r.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	method, ppID := PickupFor(r)
	for i := range cs {
		if cs[i].Status != claim.StatusApproved {
			continue
		}
		cs[i].RefreshPickup(method, ppID)
		if err := s.claims.Update(ctx, &cs[i]); err != nil {
			return apperror.Internal(err)
		}
		if err := s.notifier.PickupChanged(ctx, &cs[i], r, s.now()); err != nil {
			return apperror.Internal(err)
		}
	}
	return nil
}

// PickupFor: pickup follows the item's actual location (PRD 1.3).
func PickupFor(r *domain.Report) (claim.PickupMethod, *uuid.UUID) {
	if r.IsCountedAsStock() {
		id := *r.PickupPointID
		return claim.PickupAtPoint, &id
	}
	return claim.PickupMeetFinder, nil
}

// applyLocationPoint links the report to a pos satpam/fakultas: area = faculty, and the
// point's coordinates/name fill in whatever the reporter left empty.
func (s *Service) applyLocationPoint(ctx context.Context, r *domain.Report, id uuid.UUID) error {
	p, err := s.pickups.FindByID(ctx, id)
	if err != nil || !p.IsActive {
		if err != nil && !errors.Is(err, pickup.ErrNotFound) {
			return apperror.Internal(err)
		}
		return apperror.Validation(map[string]string{"location_point_id": "lokasi tidak ditemukan"})
	}
	r.LocationPointID = &p.ID
	r.LocationArea = p.Building
	if r.LocationName == "" {
		r.LocationName = p.Name
	}
	if r.Latitude == nil || r.Longitude == nil {
		lat, lng := p.Latitude, p.Longitude
		r.Latitude, r.Longitude = &lat, &lng
	}
	return nil
}
