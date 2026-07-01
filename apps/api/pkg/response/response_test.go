package response_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupApp() *fiber.App {
	return fiber.New()
}

func TestOK(t *testing.T) {
	app := setupApp()
	app.Get("/test", func(c fiber.Ctx) error {
		return response.OK(c, map[string]string{"key": "value"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	var r response.Response
	json.Unmarshal(body, &r)

	assert.Equal(t, 0, r.Code)
	assert.Equal(t, "OK", r.Message)
	assert.NotNil(t, r.Data)
}

func TestOKMessage(t *testing.T) {
	app := setupApp()
	app.Get("/test", func(c fiber.Ctx) error {
		return response.OKMessage(c, "done")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	body := readBody(t, resp)
	var r response.Response
	json.Unmarshal(body, &r)

	assert.Equal(t, 0, r.Code)
	assert.Equal(t, "done", r.Message)
	assert.Nil(t, r.Data)
}

func TestPaginated(t *testing.T) {
	app := setupApp()
	app.Get("/test", func(c fiber.Ctx) error {
		return response.Paginated(c, []int{1, 2, 3}, 100)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	body := readBody(t, resp)
	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Items []int `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	json.Unmarshal(body, &r)

	assert.Equal(t, 0, r.Code)
	assert.Equal(t, int64(100), r.Data.Total)
	assert.Equal(t, []int{1, 2, 3}, r.Data.Items)
}

func TestError(t *testing.T) {
	app := setupApp()
	app.Get("/test", func(c fiber.Ctx) error {
		return response.Error(c, errors.ErrNotFound("item not found"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	body := readBody(t, resp)
	var r response.Response
	json.Unmarshal(body, &r)

	assert.Equal(t, 40400, r.Code)
	assert.Equal(t, "item not found", r.Message)
	assert.Nil(t, r.Data)
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()
	return body
}
