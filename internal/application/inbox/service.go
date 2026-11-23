// Package inbox implements InboxStatusService (Fungsi 3): notification list,
// unread badges, read status.
package inbox

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/temuin/backend/internal/application/dto"
	"github.com/temuin/backend/internal/domain/claim"
	"github.com/temuin/backend/internal/domain/notification"
	"github.com/temuin/backend/internal/shared/apperror"
)

type Notification struct {
	ID        uuid.UUID         `json:"id"`
	Type      notification.Type `json:"type"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	ClaimID   *uuid.UUID        `json:"claim_id"`
	ReportID  *uuid.UUID        `json:"report_id"`
	IsRead    bool              `json:"is_read"`
	ReadAt    *time.Time        `json:"read_at"`
	CreatedAt time.Time         `json:"created_at"`
}

type Badge struct {
	UnreadNotifications   int64 `json:"unread_notifications"`
	PendingIncomingClaims int64 `json:"pending_incoming_claims"`
	Total                 int64 `json:"total"`
}

type Service struct {
	notifications notification.Repository
	claims        claim.Repository
}

func NewService(n notification.Repository, c claim.Repository) *Service {
	return &Service{notifications: n, claims: c}
}

func (s *Service) List(ctx context.Context, uid uuid.UUID, unreadOnly bool, page, limit int) ([]Notification, dto.Meta, error) {
	ns, total, err := s.notifications.List(ctx, uid, unreadOnly, page, limit)
	if err != nil {
		return nil, dto.Meta{}, apperror.Internal(err)
	}
	out := make([]Notification, 0, len(ns))
	for _, n := range ns {
		out = append(out, Notification{ID: n.ID, Type: n.Type, Title: n.Title, Body: n.Body, ClaimID: n.ClaimID,
			ReportID: n.ReportID, IsRead: n.ReadAt != nil, ReadAt: n.ReadAt, CreatedAt: n.CreatedAt})
	}
	return out, dto.NewMeta(page, limit, total), nil
}

func (s *Service) Badge(ctx context.Context, uid uuid.UUID) (*Badge, error) {
	unread, err := s.notifications.CountUnread(ctx, uid)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	pending := claim.StatusPending
	_, pend, err := s.claims.ListByFinder(ctx, uid, claim.ListFilter{Status: &pending, Limit: 1})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &Badge{UnreadNotifications: unread, PendingIncomingClaims: pend, Total: unread + pend}, nil
}

func (s *Service) MarkRead(ctx context.Context, uid, id uuid.UUID) error {
	err := s.notifications.MarkRead(ctx, id, uid, time.Now().UTC())
	if errors.Is(err, notification.ErrNotFound) {
		return apperror.NotFound("NOTIFICATION_NOT_FOUND", "Notifikasi tidak ditemukan")
	}
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *Service) MarkAllRead(ctx context.Context, uid uuid.UUID) (int64, error) {
	n, err := s.notifications.MarkAllRead(ctx, uid, time.Now().UTC())
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}
