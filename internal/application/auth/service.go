package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/temuin/backend/internal/application/dto"
	"github.com/temuin/backend/internal/domain/user"
	"github.com/temuin/backend/internal/shared/apperror"
)

// RegisterInput mirrors the "Daftar Akun Temuin" Figma form:
// Nama, NIM, Fakultas, WhatsApp, Kata Sandi, Ulangi Kata Sandi, persetujuan ketentuan.
// Email is optional (not on the form).
type RegisterInput struct {
	Name                 string  `json:"name" validate:"required,min=2,max=120"`
	NIM                  string  `json:"nim" validate:"required,numeric,min=8,max=20"`
	Faculty              string  `json:"faculty" validate:"required,max=120"`
	WhatsApp             string  `json:"whatsapp" validate:"required,min=8,max=30"`
	Password             string  `json:"password" validate:"required,min=8,max=72"`
	PasswordConfirmation string  `json:"password_confirmation" validate:"required,eqfield=Password"`
	AgreeTerms           bool    `json:"agree_terms" validate:"required"`
	Email                *string `json:"email" validate:"omitempty,email,max=160"`
	Instagram            *string `json:"instagram" validate:"omitempty,max=60"`
}

// LoginInput: identifier is NIM or email. `email` is kept as a legacy alias.
type LoginInput struct {
	Identifier string `json:"identifier" validate:"required_without=Email,max=160"`
	Email      string `json:"email" validate:"omitempty,max=160"`
	Password   string `json:"password" validate:"required"`
}

type Result struct {
	AccessToken string          `json:"access_token"`
	TokenType   string          `json:"token_type"`
	ExpiresAt   time.Time       `json:"expires_at"`
	User        dto.UserPrivate `json:"user"`
}

type Service struct {
	users  user.Repository
	hasher user.PasswordHasher
	tokens user.TokenIssuer
}

func NewService(u user.Repository, h user.PasswordHasher, t user.TokenIssuer) *Service {
	return &Service{users: u, hasher: h, tokens: t}
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*Result, error) {
	faculty, ok := user.NormalizeFaculty(strings.TrimSpace(in.Faculty))
	if !ok {
		return nil, apperror.Validation(map[string]string{"faculty": "fakultas tidak dikenal"})
	}
	nim := strings.TrimSpace(in.NIM)
	if _, err := s.users.FindByNIM(ctx, nim); err == nil {
		return nil, apperror.Conflict("NIM_TAKEN", "NIM sudah terdaftar")
	} else if !errors.Is(err, user.ErrNotFound) {
		return nil, apperror.Internal(err)
	}
	var email *string
	if in.Email != nil && strings.TrimSpace(*in.Email) != "" {
		e := strings.ToLower(strings.TrimSpace(*in.Email))
		if _, err := s.users.FindByEmail(ctx, e); err == nil {
			return nil, apperror.Conflict("EMAIL_TAKEN", "Email sudah terdaftar")
		} else if !errors.Is(err, user.ErrNotFound) {
			return nil, apperror.Internal(err)
		}
		email = &e
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	wa := strings.TrimSpace(in.WhatsApp)
	u := user.New(strings.TrimSpace(in.Name), email, hash)
	u.NIM, u.Faculty, u.WhatsApp, u.Instagram = &nim, &faculty, &wa, in.Instagram
	if err := s.users.Create(ctx, u); err != nil {
		if errors.Is(err, user.ErrEmailTaken) {
			return nil, apperror.Conflict("NIM_TAKEN", "NIM atau email sudah terdaftar")
		}
		return nil, apperror.Internal(err)
	}
	return s.issue(u)
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*Result, error) {
	id := strings.TrimSpace(in.Identifier)
	if id == "" {
		id = strings.TrimSpace(in.Email)
	}
	var (
		u   *user.User
		err error
	)
	if strings.Contains(id, "@") {
		u, err = s.users.FindByEmail(ctx, id)
	} else {
		u, err = s.users.FindByNIM(ctx, id)
	}
	if err != nil || !s.hasher.Compare(u.PasswordHash, in.Password) {
		if err != nil && !errors.Is(err, user.ErrNotFound) {
			return nil, apperror.Internal(err)
		}
		return nil, apperror.Unauthorized("INVALID_CREDENTIALS", "NIM/email atau kata sandi salah")
	}
	return s.issue(u)
}

func (s *Service) issue(u *user.User) (*Result, error) {
	tok, exp, err := s.tokens.Issue(u.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &Result{AccessToken: tok, TokenType: "Bearer", ExpiresAt: exp, User: dto.ToUserPrivate(u)}, nil
}
