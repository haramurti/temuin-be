package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/temuin/backend/internal/domain/user"
	"github.com/temuin/backend/internal/shared/apperror"
)

const userKey = "uid"

// Auth requires a valid Bearer JWT. All non-auth endpoints are behind it
// (PRD: data publik hanya dapat dibaca pengguna yang sudah masuk).
func Auth(tokens user.TokenIssuer) fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get(fiber.HeaderAuthorization)
		if !strings.HasPrefix(h, "Bearer ") {
			return apperror.Unauthorized("UNAUTHENTICATED", "Token akses diperlukan")
		}
		uid, err := tokens.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			return apperror.Unauthorized("INVALID_TOKEN", "Token tidak valid atau kedaluwarsa")
		}
		c.Locals(userKey, uid)
		return c.Next()
	}
}

func UserID(c *fiber.Ctx) uuid.UUID {
	if v, ok := c.Locals(userKey).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}
