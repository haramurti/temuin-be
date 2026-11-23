// Package response defines the JSON envelope used by every endpoint.
package response

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/temuin/backend/internal/shared/apperror"
)

type Body struct {
	Success bool       `json:"success"`
	Message string     `json:"message,omitempty"`
	Data    any        `json:"data,omitempty"`
	Meta    any        `json:"meta,omitempty"`
	Error   *ErrorBody `json:"error,omitempty"`
}

type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func OK(c *fiber.Ctx, msg string, data any) error {
	return c.Status(fiber.StatusOK).JSON(Body{Success: true, Message: msg, Data: data})
}

func Created(c *fiber.Ctx, msg string, data any) error {
	return c.Status(fiber.StatusCreated).JSON(Body{Success: true, Message: msg, Data: data})
}

func Paged(c *fiber.Ctx, msg string, data any, meta any) error {
	return c.Status(fiber.StatusOK).JSON(Body{Success: true, Message: msg, Data: data, Meta: meta})
}

func status(k apperror.Kind) int {
	switch k {
	case apperror.KindValidation:
		return fiber.StatusUnprocessableEntity
	case apperror.KindBadRequest:
		return fiber.StatusBadRequest
	case apperror.KindUnauthorized:
		return fiber.StatusUnauthorized
	case apperror.KindForbidden:
		return fiber.StatusForbidden
	case apperror.KindNotFound:
		return fiber.StatusNotFound
	case apperror.KindConflict:
		return fiber.StatusConflict
	case apperror.KindUnprocessable:
		return fiber.StatusUnprocessableEntity
	case apperror.KindTooManyRequests:
		return fiber.StatusTooManyRequests
	}
	return fiber.StatusInternalServerError
}

// ErrorHandler is Fiber's global error handler.
func ErrorHandler(c *fiber.Ctx, err error) error {
	var ae *apperror.Error
	if errors.As(err, &ae) {
		if ae.Kind == apperror.KindInternal {
			log.Printf("internal error: %v", ae)
		}
		return c.Status(status(ae.Kind)).JSON(Body{Error: &ErrorBody{Code: ae.Code, Message: ae.Message, Details: ae.Details}})
	}
	var fe *fiber.Error
	if errors.As(err, &fe) {
		code := "HTTP_ERROR"
		switch fe.Code {
		case fiber.StatusNotFound:
			code = "ROUTE_NOT_FOUND"
		case fiber.StatusMethodNotAllowed:
			code = "METHOD_NOT_ALLOWED"
		case fiber.StatusRequestEntityTooLarge:
			code = "PAYLOAD_TOO_LARGE"
		}
		return c.Status(fe.Code).JSON(Body{Error: &ErrorBody{Code: code, Message: fe.Message}})
	}
	log.Printf("unhandled error: %v", err)
	return c.Status(fiber.StatusInternalServerError).JSON(Body{Error: &ErrorBody{Code: "INTERNAL_ERROR", Message: "Terjadi kesalahan pada server"}})
}
