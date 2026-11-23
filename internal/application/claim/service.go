// Package claim implements ClaimService (Fungsi 1): validateClaim, approveClaim,
// rejectClaim, completeHandover — plus inbox views (Fungsi 3).
package claim

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/temuin/backend/internal/application/dto"
	"github.com/temuin/backend/internal/application/events"
	reportapp "github.com/temuin/backend/internal/application/report"
	domain "github.com/temuin/backend/internal/domain/claim"
	"github.com/temuin/backend/internal/domain/notification"
	"github.com/temuin/backend/internal/domain/pickup"
	"github.com/temuin/backend/internal/domain/report"
	"github.com/temuin/backend/internal/domain/user"
	"github.com/temuin/backend/internal/shared/apperror"
	"github.com/temuin/backend/internal/shared/tx"
)

type SubmitInput struct {
	SecretDescription string  `json:"secret_description" validate:"required,min=10,max=2000"`
	ProofPhotoURL     *string `json:"proof_photo_url" validate:"omitempty,url,max=512"`
}

type RejectInput struct {
	Reason string `json:"reason" validate:"required,min=3,max=500"`
}

type Action string

const (
	ActionApprove          Action = "APPROVE"
	ActionReject           Action = "REJECT"
	ActionCancel           Action = "CANCEL"
	ActionResubmit         Action = "RESUBMIT"
	ActionCompleteHandover Action = "COMPLETE_HANDOVER"
	ActionViewProof        Action = "VIEW_PROOF"
	ActionContact          Action = "CONTACT"
)

type ReportSummary struct {
	ID       uuid.UUID       `json:"id"`
	Type     report.Type     `json:"type"`
	Title    string          `json:"title"`
	Category report.Category `json:"category"`
	PhotoURL *string         `json:"photo_url"`
	Status   report.Status   `json:"status"`
	Custody  *report.Custody `json:"custody"`
}

type Pickup struct {
	Method       *domain.PickupMethod `json:"method"`
	PickupPoint  *dto.PickupPoint     `json:"pickup_point"`
	Instructions string               `json:"instructions"`
}

type Claim struct {
	ID                uuid.UUID       `json:"id"`
	Role              string          `json:"role"` // FINDER | CLAIMANT (viewer's role)
	Status            domain.Status   `json:"status"`
	Report            ReportSummary   `json:"report"`
	Claimant          *dto.UserPublic `json:"claimant"`
	Finder            *dto.UserPublic `json:"finder"`
	SecretDescription string          `json:"secret_description"`
	ProofPhotoURL     *string         `json:"proof_photo_url"`
	RejectionReason   *string         `json:"rejection_reason"`
	Pickup            *Pickup         `json:"pickup"`
	PreviousClaimID   *uuid.UUID      `json:"previous_claim_id"`
	AllowedActions    []Action        `json:"allowed_actions"`
	DecidedAt         *time.Time      `json:"decided_at"`
	CompletedAt       *time.Time      `json:"completed_at"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type Proof struct {
	ClaimID     uuid.UUID     `json:"claim_id"`
	ReportID    uuid.UUID     `json:"report_id"`
	Status      domain.Status `json:"status"`
	Item        ReportSummary `json:"item"`
	Claimant    ProofPerson   `json:"claimant"`
	Finder      ProofPerson   `json:"finder"`
	Pickup      Pickup        `json:"pickup"`
	ApprovedAt  *time.Time    `json:"approved_at"`
	CompletedAt *time.Time    `json:"completed_at"`
	Disclaimer  string        `json:"disclaimer"`
}

type ProofPerson struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	NIM     *string   `json:"nim"`
	Faculty *string   `json:"faculty"`
}

type Service struct {
	tx            tx.Manager
	claims        domain.Repository
	reports       report.Repository
	users         user.Repository
	pickups       pickup.Repository
	notifications notification.Repository
	notifier      *events.Notifier
	now           func() time.Time
}

func NewService(t tx.Manager, c domain.Repository, r report.Repository, u user.Repository, p pickup.Repository,
	n notification.Repository, nf *events.Notifier) *Service {
	return &Service{tx: t, claims: c, reports: r, users: u, pickups: p, notifications: n, notifier: nf,
		now: func() time.Time { return time.Now().UTC() }}
}

var (
	errClaimNotFound = apperror.NotFound("CLAIM_NOT_FOUND", "Klaim tidak ditemukan")
	errReportNF      = apperror.NotFound("REPORT_NOT_FOUND", "Laporan tidak ditemukan")
	errItemGone      = apperror.Unprocessable("ITEM_UNAVAILABLE", "Barang sudah tidak tersedia untuk diklaim")
)

func mapErr(err error) error {
	var ae *apperror.Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ae):
		return ae
	case errors.Is(err, domain.ErrNotFound):
		return errClaimNotFound
	case errors.Is(err, report.ErrNotFound):
		return errReportNF
	case errors.Is(err, domain.ErrNotFinder):
		return apperror.Forbidden("NOT_FINDER", "Hanya pengunggah laporan yang dapat memproses klaim")
	case errors.Is(err, domain.ErrNotClaimant):
		return apperror.Forbidden("NOT_CLAIMANT", "Hanya pengaju klaim yang dapat melakukan aksi ini")
	case errors.Is(err, domain.ErrInvalidState):
		return apperror.Conflict("INVALID_CLAIM_STATE", "Status klaim sudah berubah; muat ulang data")
	case errors.Is(err, domain.ErrReasonRequired):
		return apperror.Validation(map[string]string{"reason": "wajib diisi"})
	case errors.Is(err, domain.ErrConflict):
		return apperror.Conflict("CLAIM_CONFLICT", "Klaim bentrok dengan klaim lain (sudah ada klaim aktif/disetujui)")
	case errors.Is(err, report.ErrNotActive):
		return errItemGone
	}
	return apperror.Internal(err)
}

/* ----- validateClaim + submit ----- */

func (s *Service) Submit(ctx context.Context, uid, reportID uuid.UUID, in SubmitInput) (*Claim, error) {
	return s.submit(ctx, uid, reportID, in, nil)
}

func (s *Service) submit(ctx context.Context, uid, reportID uuid.UUID, in SubmitInput, previous *uuid.UUID) (*Claim, error) {
	var created *domain.Claim
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.reports.FindByIDForUpdate(ctx, reportID)
		if err != nil {
			return err
		}
		if r.Type != report.TypeFound {
			return apperror.Unprocessable("FOUND_REPORT_ONLY", "Klaim hanya dapat diajukan pada laporan barang ditemukan")
		}
		if r.IsOwnedBy(uid) {
			return apperror.Forbidden("SELF_CLAIM", "Tidak dapat mengajukan klaim atas laporan sendiri")
		}
		if !r.IsAvailableForClaim() {
			return errItemGone
		}
		if ok, err := s.claims.HasApproved(ctx, r.ID); err != nil {
			return err
		} else if ok {
			return apperror.Conflict("ALREADY_APPROVED", "Barang ini sudah memiliki klaim yang disetujui")
		}
		if ok, err := s.claims.HasOpenClaim(ctx, r.ID, uid); err != nil {
			return err
		} else if ok {
			return apperror.Conflict("DUPLICATE_CLAIM", "Kamu sudah memiliki klaim aktif untuk barang ini")
		}
		c := domain.New(r.ID, uid, r.UserID, strings.TrimSpace(in.SecretDescription), in.ProofPhotoURL, previous)
		if err := s.claims.Create(ctx, c); err != nil {
			return err
		}
		created = c
		return s.notifier.ClaimStatusChanged(ctx, c, r)
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return s.Get(ctx, uid, created.ID)
}

// Resubmit: after rejection, claimant submits again with new evidence (new claim, linked).
func (s *Service) Resubmit(ctx context.Context, uid, claimID uuid.UUID, in SubmitInput) (*Claim, error) {
	old, err := s.claims.FindByID(ctx, claimID)
	if err != nil {
		return nil, mapErr(err)
	}
	if old.ClaimantID != uid {
		return nil, mapErr(domain.ErrNotClaimant)
	}
	if !old.CanResubmit() {
		return nil, apperror.Conflict("CANNOT_RESUBMIT", "Hanya klaim yang ditolak yang dapat diajukan ulang")
	}
	if strings.TrimSpace(in.SecretDescription) == strings.TrimSpace(old.SecretDescription) &&
		(in.ProofPhotoURL == nil || (old.ProofPhotoURL != nil && *in.ProofPhotoURL == *old.ProofPhotoURL)) {
		return nil, apperror.Validation(map[string]string{"secret_description": "ajukan ulang dengan bukti/ciri baru"})
	}
	return s.submit(ctx, uid, old.ReportID, in, &old.ID)
}

/* ----- decisions ----- */

// withLocked loads report then claim with row locks (consistent lock order).
func (s *Service) withLocked(ctx context.Context, claimID uuid.UUID, fn func(ctx context.Context, c *domain.Claim, r *report.Report) error) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		c0, err := s.claims.FindByID(ctx, claimID)
		if err != nil {
			return err
		}
		r, err := s.reports.FindByIDForUpdate(ctx, c0.ReportID)
		if err != nil {
			return err
		}
		c, err := s.claims.FindByIDForUpdate(ctx, claimID)
		if err != nil {
			return err
		}
		return fn(ctx, c, r)
	})
}

func (s *Service) Approve(ctx context.Context, uid, claimID uuid.UUID) (*Claim, error) {
	err := s.withLocked(ctx, claimID, func(ctx context.Context, c *domain.Claim, r *report.Report) error {
		if c.FinderID != uid {
			return domain.ErrNotFinder
		}
		if !r.IsAvailableForClaim() {
			if c.MarkItemUnavailable() {
				if err := s.claims.Update(ctx, c); err != nil {
					return err
				}
				_ = s.notifier.ClaimStatusChanged(ctx, c, r)
			}
			return nil // committed; caller sees ITEM_UNAVAILABLE status
		}
		if ok, err := s.claims.HasApproved(ctx, r.ID); err != nil {
			return err
		} else if ok {
			return apperror.Conflict("ALREADY_APPROVED", "Sudah ada klaim lain yang disetujui untuk barang ini")
		}
		method, ppID := reportapp.PickupFor(r)
		if err := c.Approve(uid, method, ppID, s.now()); err != nil {
			return err
		}
		if err := s.claims.Update(ctx, c); err != nil {
			return err
		}
		return s.notifier.ClaimStatusChanged(ctx, c, r)
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return s.Get(ctx, uid, claimID)
}

func (s *Service) Reject(ctx context.Context, uid, claimID uuid.UUID, in RejectInput) (*Claim, error) {
	err := s.withLocked(ctx, claimID, func(ctx context.Context, c *domain.Claim, r *report.Report) error {
		if err := c.Reject(uid, strings.TrimSpace(in.Reason), s.now()); err != nil {
			return err
		}
		if err := s.claims.Update(ctx, c); err != nil {
			return err
		}
		return s.notifier.ClaimStatusChanged(ctx, c, r)
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return s.Get(ctx, uid, claimID)
}

func (s *Service) Cancel(ctx context.Context, uid, claimID uuid.UUID) (*Claim, error) {
	err := s.withLocked(ctx, claimID, func(ctx context.Context, c *domain.Claim, r *report.Report) error {
		if err := c.Cancel(uid); err != nil {
			return err
		}
		if err := s.claims.Update(ctx, c); err != nil {
			return err
		}
		return s.notifier.ClaimStatusChanged(ctx, c, r)
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return s.Get(ctx, uid, claimID)
}

// CompleteHandover: finder confirms physical handover; report & stock no longer available.
func (s *Service) CompleteHandover(ctx context.Context, uid, claimID uuid.UUID) (*Claim, error) {
	err := s.withLocked(ctx, claimID, func(ctx context.Context, c *domain.Claim, r *report.Report) error {
		now := s.now()
		if err := c.Complete(uid, now); err != nil {
			return err
		}
		if err := r.Complete(now); err != nil {
			return err
		}
		if err := s.claims.Update(ctx, c); err != nil {
			return err
		}
		if err := s.reports.Update(ctx, r); err != nil {
			return err
		}
		if err := s.notifier.ClaimStatusChanged(ctx, c, r); err != nil {
			return err
		}
		others, err := s.claims.ListByReport(ctx, r.ID)
		if err != nil {
			return err
		}
		for i := range others {
			if others[i].ID != c.ID && others[i].MarkItemUnavailable() {
				if err := s.claims.Update(ctx, &others[i]); err != nil {
					return err
				}
				if err := s.notifier.ClaimStatusChanged(ctx, &others[i], r); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return s.Get(ctx, uid, claimID)
}

/* ----- reads ----- */

func (s *Service) Get(ctx context.Context, uid, claimID uuid.UUID) (*Claim, error) {
	c, err := s.claims.FindByID(ctx, claimID)
	if err != nil {
		return nil, mapErr(err)
	}
	if !c.IsParty(uid) {
		// do not reveal existence of claims to non-parties
		return nil, errClaimNotFound
	}
	out, err := s.build(ctx, uid, []domain.Claim{*c})
	if err != nil {
		return nil, err
	}
	_ = s.notifications.MarkReadByClaim(ctx, c.ID, uid, s.now())
	return &out[0], nil
}

func (s *Service) Proof(ctx context.Context, uid, claimID uuid.UUID) (*Proof, error) {
	c, err := s.claims.FindByID(ctx, claimID)
	if err != nil {
		return nil, mapErr(err)
	}
	if !c.IsParty(uid) {
		return nil, errClaimNotFound
	}
	if c.Status != domain.StatusApproved && c.Status != domain.StatusCompleted {
		return nil, apperror.Unprocessable("PROOF_UNAVAILABLE", "Bukti persetujuan hanya tersedia untuk klaim yang disetujui")
	}
	r, err := s.reports.FindByID(ctx, c.ReportID)
	if err != nil {
		return nil, mapErr(err)
	}
	us, err := s.users.FindByIDs(ctx, []uuid.UUID{c.ClaimantID, c.FinderID})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	pk, err := s.pickupInfo(ctx, c, r)
	if err != nil {
		return nil, err
	}
	if pk == nil {
		pk = &Pickup{}
	}
	person := func(u *user.User) ProofPerson {
		if u == nil {
			return ProofPerson{}
		}
		return ProofPerson{ID: u.ID, Name: u.Name, NIM: u.NIM, Faculty: u.Faculty}
	}
	return &Proof{
		ClaimID: c.ID, ReportID: r.ID, Status: c.Status, Item: summary(r),
		Claimant: person(us[c.ClaimantID]), Finder: person(us[c.FinderID]), Pickup: *pk,
		ApprovedAt: c.DecidedAt, CompletedAt: c.CompletedAt,
		Disclaimer: "Bukti ini merupakan persetujuan di aplikasi Temuin, bukan pengesahan universitas. Petugas memeriksa bukti bersama barang dan identitas penerima.",
	}, nil
}

func (s *Service) ListIncoming(ctx context.Context, uid uuid.UUID, f domain.ListFilter) ([]Claim, dto.Meta, error) {
	cs, total, err := s.claims.ListByFinder(ctx, uid, f)
	if err != nil {
		return nil, dto.Meta{}, apperror.Internal(err)
	}
	out, err := s.build(ctx, uid, cs)
	return out, dto.NewMeta(f.Page, f.Limit, total), err
}

func (s *Service) ListMine(ctx context.Context, uid uuid.UUID, f domain.ListFilter) ([]Claim, dto.Meta, error) {
	cs, total, err := s.claims.ListByClaimant(ctx, uid, f)
	if err != nil {
		return nil, dto.Meta{}, apperror.Internal(err)
	}
	out, err := s.build(ctx, uid, cs)
	return out, dto.NewMeta(f.Page, f.Limit, total), err
}

// ListForReport: finder views all claims on their own report.
func (s *Service) ListForReport(ctx context.Context, uid, reportID uuid.UUID) ([]Claim, error) {
	r, err := s.reports.FindByID(ctx, reportID)
	if err != nil {
		return nil, mapErr(err)
	}
	if !r.IsOwnedBy(uid) {
		return nil, apperror.Forbidden("NOT_REPORT_OWNER", "Hanya pengunggah laporan yang dapat melihat klaim")
	}
	cs, err := s.claims.ListByReport(ctx, reportID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return s.build(ctx, uid, cs)
}

func summary(r *report.Report) ReportSummary {
	return ReportSummary{ID: r.ID, Type: r.Type, Title: r.Title, Category: r.Category, PhotoURL: r.PhotoURL, Status: r.Status, Custody: r.Custody}
}

func (s *Service) pickupInfo(ctx context.Context, c *domain.Claim, r *report.Report) (*Pickup, error) {
	p := &Pickup{Method: c.PickupMethod}
	if c.PickupMethod == nil {
		return nil, nil
	}
	if *c.PickupMethod == domain.PickupAtPoint && c.PickupPointID != nil {
		pp, err := s.pickups.FindByID(ctx, *c.PickupPointID)
		if err != nil && !errors.Is(err, pickup.ErrNotFound) {
			return nil, apperror.Internal(err)
		}
		if err == nil {
			p.PickupPoint = dto.ToPickupPoint(pp)
			p.Instructions = "Ambil barang di " + pp.Name + ". Tunjukkan bukti persetujuan dan identitas kepada petugas."
		}
	} else {
		p.Instructions = "Barang masih dipegang penemu. Atur pertemuan langsung (lokasi kejadian: " + r.LocationName + ")."
	}
	return p, nil
}

// build maps claims to DTOs with role-based allowed actions (resolveAllowedActions).
func (s *Service) build(ctx context.Context, uid uuid.UUID, cs []domain.Claim) ([]Claim, error) {
	rids, uids := []uuid.UUID{}, []uuid.UUID{}
	for _, c := range cs {
		rids = append(rids, c.ReportID)
		uids = append(uids, c.ClaimantID, c.FinderID)
	}
	rs, err := s.reports.FindByIDs(ctx, rids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	us, err := s.users.FindByIDs(ctx, uids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]Claim, 0, len(cs))
	for i := range cs {
		c := &cs[i]
		r := rs[c.ReportID]
		if r == nil {
			continue
		}
		role := "CLAIMANT"
		if c.FinderID == uid {
			role = "FINDER"
		}
		pk, err := s.pickupInfo(ctx, c, r)
		if err != nil {
			return nil, err
		}
		out = append(out, Claim{
			ID: c.ID, Role: role, Status: c.Status, Report: summary(r),
			Claimant: dto.ToUserPublic(us[c.ClaimantID]), Finder: dto.ToUserPublic(us[c.FinderID]),
			SecretDescription: c.SecretDescription, ProofPhotoURL: c.ProofPhotoURL, RejectionReason: c.RejectionReason,
			Pickup: pk, PreviousClaimID: c.PreviousClaimID, AllowedActions: AllowedActions(c, r, uid),
			DecidedAt: c.DecidedAt, CompletedAt: c.CompletedAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		})
	}
	return out, nil
}

// AllowedActions is the role × status action matrix.
func AllowedActions(c *domain.Claim, r *report.Report, uid uuid.UUID) []Action {
	acts := []Action{}
	isFinder, isClaimant := c.FinderID == uid, c.ClaimantID == uid
	switch c.Status {
	case domain.StatusPending:
		if isFinder && r.IsAvailableForClaim() {
			acts = append(acts, ActionApprove, ActionReject)
		}
		if isClaimant {
			acts = append(acts, ActionCancel)
		}
	case domain.StatusApproved:
		acts = append(acts, ActionViewProof, ActionContact)
		if isFinder {
			acts = append(acts, ActionCompleteHandover)
		}
	case domain.StatusRejected:
		if isClaimant && r.IsAvailableForClaim() {
			acts = append(acts, ActionResubmit)
		}
	case domain.StatusCompleted:
		acts = append(acts, ActionViewProof)
	}
	return acts
}
