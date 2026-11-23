// Package pickup holds the developer-configured pick-up points (pos penitipan).
package pickup

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("pickup point not found")

type PickupPoint struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code         string    `gorm:"size:40;uniqueIndex;not null"`
	Name         string    `gorm:"size:120;not null"`
	Description  *string
	Building     *string `gorm:"size:120"`
	Latitude     float64 `gorm:"not null"`
	Longitude    float64 `gorm:"not null"`
	OpeningHours *string `gorm:"size:120"`
	IsActive     bool    `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Repository interface {
	List(ctx context.Context, activeOnly bool) ([]PickupPoint, error)
	FindByID(ctx context.Context, id uuid.UUID) (*PickupPoint, error)
	FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*PickupPoint, error)
}
