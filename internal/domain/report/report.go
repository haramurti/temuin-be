// Package report is the lost & found report bounded context.
package report

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeLost  Type = "LOST"
	TypeFound Type = "FOUND"
)

func (t Type) Valid() bool { return t == TypeLost || t == TypeFound }
func (t Type) Opposite() Type {
	if t == TypeLost {
		return TypeFound
	}
	return TypeLost
}

type Status string

const (
	StatusActive    Status = "ACTIVE"    // visible & actionable
	StatusCompleted Status = "COMPLETED" // handed over / resolved
	StatusClosed    Status = "CLOSED"    // closed by owner without handover
)

// Custody only applies to FOUND reports.
type Custody string

const (
	CustodyHeldByFinder    Custody = "HELD_BY_FINDER"   // masih dipegang penemu
	CustodyAwaitingDropoff Custody = "AWAITING_DROPOFF" // Menunggu Penitipan
	CustodyDeposited       Custody = "DEPOSITED"        // Dititipkan (counted as stock)
	CustodyUnavailable     Custody = "UNAVAILABLE"      // tidak lagi tersedia
)

func (c Custody) Valid() bool {
	switch c {
	case CustodyHeldByFinder, CustodyAwaitingDropoff, CustodyDeposited, CustodyUnavailable:
		return true
	}
	return false
}

type Category string

const (
	CategoryElectronics Category = "ELECTRONICS"
	CategoryWallet      Category = "WALLET"
	CategoryIDCard      Category = "ID_CARD"
	CategoryKeys        Category = "KEYS"
	CategoryBag         Category = "BAG"
	CategoryClothing    Category = "CLOTHING"
	CategoryAccessories Category = "ACCESSORIES"
	CategoryBooks       Category = "BOOKS"
	CategoryStationery  Category = "STATIONERY"
	CategoryBottle      Category = "BOTTLE"
	CategoryOther       Category = "OTHER"
)

var Categories = []Category{
	CategoryElectronics, CategoryWallet, CategoryIDCard, CategoryKeys, CategoryBag, CategoryClothing,
	CategoryAccessories, CategoryBooks, CategoryStationery, CategoryBottle, CategoryOther,
}

func (c Category) Valid() bool {
	for _, x := range Categories {
		if x == c {
			return true
		}
	}
	return false
}

var (
	ErrNotFound            = errors.New("report not found")
	ErrNotFoundReport      = ErrNotFound
	ErrNotFoundType        = errors.New("operation only valid for FOUND reports")
	ErrNotActive           = errors.New("report is not active")
	ErrInvalidCustodyState = errors.New("invalid custody transition")
	ErrPickupPointRequired = errors.New("pickup point is required")
)

// Report is the aggregate root for a lost/found item post.
type Report struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index"`
	Type         Type      `gorm:"size:10;not null;index"`
	Title        string    `gorm:"size:120;not null"`
	Category     Category  `gorm:"size:30;not null;index"`
	Color        *string   `gorm:"size:40"`
	Description  string    `gorm:"type:text;not null"`
	OccurredAt   time.Time `gorm:"not null;index"`
	LocationName string    `gorm:"size:160;not null"`
	LocationArea *string   `gorm:"size:120"`
	// LocationPointID: optional pos satpam/fakultas picked in the Lapor form's "Lokasi" dropdown.
	LocationPointID *uuid.UUID `gorm:"type:uuid;index"`
	Latitude        *float64
	Longitude       *float64
	PhotoURL        *string    `gorm:"size:512"`
	Status          Status     `gorm:"size:20;not null;index"`
	Custody         *Custody   `gorm:"size:20;index"`
	PickupPointID   *uuid.UUID `gorm:"type:uuid;index"`
	DepositedAt     *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time `gorm:"index"`
	UpdatedAt       time.Time
}

func (r *Report) IsOwnedBy(uid uuid.UUID) bool { return r.UserID == uid }

// IsAvailableForClaim: claims only on active FOUND reports whose item is still available.
func (r *Report) IsAvailableForClaim() bool {
	return r.Type == TypeFound && r.Status == StatusActive &&
		(r.Custody == nil || *r.Custody != CustodyUnavailable)
}

// IsCountedAsStock: only confirmed deposits count on the map (Fungsi 2).
func (r *Report) IsCountedAsStock() bool {
	return r.Type == TypeFound && r.Status == StatusActive && r.Custody != nil &&
		*r.Custody == CustodyDeposited && r.PickupPointID != nil
}

func custodyPtr(c Custody) *Custody { return &c }

// SetHeldByFinder: the finder keeps the item; removes it from pos stock.
func (r *Report) SetHeldByFinder() error {
	if err := r.ensureActiveFound(); err != nil {
		return err
	}
	r.Custody = custodyPtr(CustodyHeldByFinder)
	r.PickupPointID = nil
	r.DepositedAt = nil
	return nil
}

// RequestDeposit selects (or changes) a pos. Not counted until confirmed.
func (r *Report) RequestDeposit(pickupPointID uuid.UUID) error {
	if err := r.ensureActiveFound(); err != nil {
		return err
	}
	if pickupPointID == uuid.Nil {
		return ErrPickupPointRequired
	}
	r.Custody = custodyPtr(CustodyAwaitingDropoff)
	r.PickupPointID = &pickupPointID
	r.DepositedAt = nil
	return nil
}

// ConfirmDeposit: finder confirms the item physically arrived at the pos.
func (r *Report) ConfirmDeposit(now time.Time) error {
	if err := r.ensureActiveFound(); err != nil {
		return err
	}
	if r.Custody == nil || *r.Custody != CustodyAwaitingDropoff || r.PickupPointID == nil {
		return ErrInvalidCustodyState
	}
	r.Custody = custodyPtr(CustodyDeposited)
	r.DepositedAt = &now
	return nil
}

// MarkUnavailable: item moved / gone / no longer at pos.
func (r *Report) MarkUnavailable() error {
	if err := r.ensureActiveFound(); err != nil {
		return err
	}
	r.Custody = custodyPtr(CustodyUnavailable)
	r.DepositedAt = nil
	return nil
}

func (r *Report) Complete(now time.Time) error {
	if r.Status != StatusActive {
		return ErrNotActive
	}
	r.Status = StatusCompleted
	r.CompletedAt = &now
	return nil
}

func (r *Report) Close(now time.Time) error {
	if r.Status != StatusActive {
		return ErrNotActive
	}
	r.Status = StatusClosed
	r.CompletedAt = &now
	return nil
}

func (r *Report) ensureActiveFound() error {
	if r.Type != TypeFound {
		return ErrNotFoundType
	}
	if r.Status != StatusActive {
		return ErrNotActive
	}
	return nil
}

type Filter struct {
	Query         string
	Type          *Type
	Category      *Category
	Status        *Status
	Custody       *Custody
	UserID        *uuid.UUID
	PickupPointID *uuid.UUID
	Page          int
	Limit         int
}

type CandidateQuery struct {
	Type          Type
	Category      Category
	ExcludeUserID uuid.UUID
	From, To      time.Time
}

type Repository interface {
	Create(ctx context.Context, r *Report) error
	Update(ctx context.Context, r *Report) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (*Report, error)
	FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*Report, error)
	FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Report, error)
	List(ctx context.Context, f Filter) ([]Report, int64, error)
	FindCandidates(ctx context.Context, q CandidateQuery) ([]Report, error)
	CountDepositedByPickupPoint(ctx context.Context) (map[uuid.UUID]int64, error)
}
