package utils_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testDTO struct {
	Name  string `json:"name" validate:"required,min=1,max=17"`
	Email string `json:"email" validate:"required,email"`
	Page  int    `json:"page" validate:"required,min=1"`
}

func TestParseAndValidate_Valid(t *testing.T) {
	app := fiber.New()
	app.Post("/test", func(c fiber.Ctx) error {
		var dto testDTO
		if err := utils.ParseAndValidate(c, &dto); err != nil {
			return c.Status(400).SendString(err.Error())
		}
		return c.SendString(dto.Name)
	})

	body := `{"name":"test","email":"a@b.com","page":1}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestParseAndValidate_MissingRequired(t *testing.T) {
	app := fiber.New()
	app.Post("/test", func(c fiber.Ctx) error {
		var dto testDTO
		if err := utils.ParseAndValidate(c, &dto); err != nil {
			return c.Status(400).SendString(err.Error())
		}
		return c.SendString("ok")
	})

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestParseAndValidate_InvalidEmail(t *testing.T) {
	app := fiber.New()
	app.Post("/test", func(c fiber.Ctx) error {
		var dto testDTO
		if err := utils.ParseAndValidate(c, &dto); err != nil {
			return c.Status(400).SendString(err.Error())
		}
		return c.SendString("ok")
	})

	body := `{"name":"test","email":"not-email","page":1}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

type queryDTO struct {
	Page  int    `query:"page" validate:"required,min=1"`
	Limit int    `query:"limit" validate:"required,min=1,max=50"`
	Sort  string `query:"sort" validate:"oneof=asc desc"`
}

func TestParseQueryAndValidate_Valid(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		var dto queryDTO
		if err := utils.ParseQueryAndValidate(c, &dto); err != nil {
			return c.Status(400).SendString(err.Error())
		}
		return c.JSON(map[string]int{"page": dto.Page, "limit": dto.Limit})
	})

	req := httptest.NewRequest(http.MethodGet, "/test?page=2&limit=10&sort=asc", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestParseQueryAndValidate_Invalid(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		var dto queryDTO
		if err := utils.ParseQueryAndValidate(c, &dto); err != nil {
			return c.Status(400).SendString(err.Error())
		}
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test?page=0&limit=100&sort=invalid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPagination_Offset(t *testing.T) {
	p := utils.Pagination{Page: 3, Limit: 10}
	assert.Equal(t, 20, p.Offset())

	p2 := utils.Pagination{Page: 1, Limit: 20}
	assert.Equal(t, 0, p2.Offset())
}
