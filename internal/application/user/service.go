package user

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/temuin/backend/internal/application/dto"
	domain "github.com/temuin/backend/internal/domain/user"
	"github.com/temuin/backend/internal/shared/apperror"
)

type UpdateProfileInput struct {
	Name      *string `json:"name" validate:"omitempty,min=2,max=120"`
	NIM       *string `json:"nim" validate:"omitempty,numeric,min=8,max=20"`
	Faculty   *string `json:"faculty" validate:"omitempty,max=120"`
	WhatsApp  *string `json:"whatsapp" validate:"omitempty,min=8,max=30"`
	Instagram *string `json:"instagram" validate:"omitempty,max=60"`
	AvatarURL *string `json:"avatar_url" validate:"omitempty,url,max=512"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=72"`
}

type Service struct {
	users  domain.Repository
	hasher domain.PasswordHasher
}

func NewService(u domain.Repository, h domain.PasswordHasher) *Service {
	return &Service{users: u, hasher: h}
}

func (s *Service) load(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, err := s.users.FindByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, apperror.NotFound("USER_NOT_FOUND", "Pengguna tidak ditemukan")
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return u, nil
}

func (s *Service) Me(ctx context.Context, id uuid.UUID) (*dto.UserPrivate, error) {
	u, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	out := dto.ToUserPrivate(u)
	return &out, nil
}

func (s *Service) Public(ctx context.Context, id uuid.UUID) (*dto.UserPublic, error) {
	u, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	return dto.ToUserPublic(u), nil
}

// emptyToNil lets clients clear an optional field by sending "".
func emptyToNil(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return &v
}

func (s *Service) UpdateMe(ctx context.Context, id uuid.UUID, in UpdateProfileInput) (*dto.UserPrivate, error) {
	u, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		u.Name = strings.TrimSpace(*in.Name)
	}
	if in.NIM != nil {
		u.NIM = emptyToNil(in.NIM)
	}
	if in.Faculty != nil {
		if f := strings.TrimSpace(*in.Faculty); f != "" {
			name, ok := domain.NormalizeFaculty(f)
			if !ok {
				return nil, apperror.Validation(map[string]string{"faculty": "fakultas tidak dikenal"})
			}
			u.Faculty = &name
		} else {
			u.Faculty = nil
		}
	}
	if in.WhatsApp != nil {
		u.WhatsApp = emptyToNil(in.WhatsApp)
	}
	if in.Instagram != nil {
		u.Instagram = emptyToNil(in.Instagram)
	}
	if in.AvatarURL != nil {
		u.AvatarURL = emptyToNil(in.AvatarURL)
	}
	if err := s.users.Update(ctx, u); err != nil {
		if errors.Is(err, domain.ErrEmailTaken) {
			return nil, apperror.Conflict("NIM_TAKEN", "NIM sudah dipakai akun lain")
		}
		return nil, apperror.Internal(err)
	}
	out := dto.ToUserPrivate(u)
	return &out, nil
}

func (s *Service) ChangePassword(ctx context.Context, id uuid.UUID, in ChangePasswordInput) error {
	u, err := s.load(ctx, id)
	if err != nil {
		return err
	}
	if !s.hasher.Compare(u.PasswordHash, in.CurrentPassword) {
		return apperror.Unprocessable("WRONG_PASSWORD", "Password saat ini salah")
	}
	h, err := s.hasher.Hash(in.NewPassword)
	if err != nil {
		return apperror.Internal(err)
	}
	u.PasswordHash = h
	if err := s.users.Update(ctx, u); err != nil {
		return apperror.Internal(err)
	}
	return nil
}
