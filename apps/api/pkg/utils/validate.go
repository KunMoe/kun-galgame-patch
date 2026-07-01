package utils

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

var validate = validator.New()

func ParseAndValidate(c fiber.Ctx, out any) error {
	if err := c.Bind().Body(out); err != nil {
		return fmt.Errorf("failed to parse request body: %w", err)
	}
	return validateStruct(out)
}

func ParseQueryAndValidate(c fiber.Ctx, out any) error {
	if err := c.Bind().Query(out); err != nil {
		return fmt.Errorf("failed to parse query parameters: %w", err)
	}
	return validateStruct(out)
}

func validateStruct(s any) error {
	if err := validate.Struct(s); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var msgs []string
			for _, e := range validationErrors {
				msgs = append(msgs, formatValidationError(e))
			}
			return fmt.Errorf("%s", strings.Join(msgs, "; "))
		}
		return err
	}
	return nil
}

func formatValidationError(e validator.FieldError) string {
	field := e.Field()
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s length must not be less than %s", field, e.Param())
	case "max":
		return fmt.Sprintf("%s length must not be greater than %s", field, e.Param())
	case "email":
		return fmt.Sprintf("%s is not a valid email address", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", field, e.Param())
	case "url":
		return fmt.Sprintf("%s is not a valid URL", field)
	default:
		return fmt.Sprintf("%s validation failed (%s)", field, e.Tag())
	}
}

func GetValidator() *validator.Validate {
	return validate
}
