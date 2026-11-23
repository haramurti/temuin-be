package handler

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/temuin/backend/internal/shared/apperror"
)

var validate = validator.New()

func init() {
	// report json field names in validation errors
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
}

// bind parses JSON and runs struct validation.
func bind(c *fiber.Ctx, out any) error {
	if err := c.BodyParser(out); err != nil {
		return apperror.BadRequest("INVALID_BODY", "Body JSON tidak valid: "+err.Error())
	}
	if err := validate.Struct(out); err != nil {
		var ves validator.ValidationErrors
		if errors.As(err, &ves) {
			d := map[string]string{}
			for _, fe := range ves {
				d[fe.Field()] = msg(fe)
			}
			return apperror.Validation(d)
		}
		return apperror.BadRequest("INVALID_BODY", err.Error())
	}
	return nil
}

func msg(fe validator.FieldError) string {
	switch fe.Tag() {
	case "eqfield":
		return "konfirmasi kata sandi tidak sama"
	case "required_without":
		return "wajib diisi"
	case "required":
		if fe.Kind() == reflect.Bool {
			return "wajib disetujui"
		}
		return "wajib diisi"
	case "email":
		return "format email tidak valid"
	case "min":
		return fmt.Sprintf("minimal %s karakter", fe.Param())
	case "max":
		return fmt.Sprintf("maksimal %s karakter", fe.Param())
	case "oneof":
		return "harus salah satu dari: " + fe.Param()
	case "url":
		return "harus berupa URL"
	case "numeric":
		return "harus berupa angka"
	case "latitude", "longitude":
		return "koordinat tidak valid"
	}
	return "tidak valid (" + fe.Tag() + ")"
}

func paramUUID(c *fiber.Ctx, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(name))
	if err != nil {
		return uuid.Nil, apperror.BadRequest("INVALID_ID", name+" harus berupa UUID")
	}
	return id, nil
}
