package handler

import (
	"errors"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	helpapp "github.com/temuin/backend/internal/application/help"
	"github.com/temuin/backend/internal/interfaces/http/response"
	"github.com/temuin/backend/internal/shared/apperror"
)

type HelpHandler struct{ svc *helpapp.Service }

func NewHelpHandler(s *helpapp.Service) *HelpHandler { return &HelpHandler{svc: s} }

type chatRequest struct {
	Message string         `json:"message"`
	History []helpapp.Turn `json:"history"`
}

func (h *HelpHandler) Topics(c *fiber.Ctx) error {
	return response.OK(c, "", h.svc.Topics())
}

func (h *HelpHandler) Chat(c *fiber.Ctx) error {
	var in chatRequest
	if err := c.BodyParser(&in); err != nil {
		return apperror.BadRequest("INVALID_BODY", "Body JSON tidak valid")
	}
	if utf8.RuneCountInString(in.Message) > helpapp.MaxMessageRunes || len(in.History) > helpapp.MaxHistoryMessages {
		return apperror.Validation(map[string]string{"message": "maksimal 500 karakter dan riwayat paling banyak dua pasang pesan"})
	}
	out, err := h.svc.Chat(c.UserContext(), in.Message, in.History)
	if err != nil {
		if errors.Is(err, helpapp.ErrInvalidInput) {
			return apperror.Validation(map[string]string{"message": "pesan wajib diisi, maksimal 500 karakter, dengan riwayat paling banyak dua pasang pesan valid"})
		}
		return apperror.Internal(err)
	}
	return response.OK(c, "", out)
}
