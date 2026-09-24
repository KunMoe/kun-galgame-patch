package userclient

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"kun-galgame-patch-api/pkg/upstream"
)

type CreatorApplication struct {
	ID            int             `json:"id"`
	UserID        int             `json:"user_id"`
	Source        string          `json:"source"`
	Status        string          `json:"status"`
	Evidence      json.RawMessage `json:"evidence,omitempty"`
	Message       string          `json:"message"`
	DeclineReason string          `json:"decline_reason"`
	ReviewedAt    *string         `json:"reviewed_at,omitempty"`
	CreatedAt     string          `json:"created_at"`
}

// The creator faces' refusals the reader can act on (infra pkg/errors/codes.go;
// creator_application_handler.go answers the 170xx as 400, and middleware.Auth
// answers a banned account 403/10014).
const (
	CreatorAlreadyHas  = 17001
	CreatorAppPending  = 17002
	CreatorAppCooldown = 17003
	AccountBanned      = 10014
)

// CreatorMessageMaxRunes is infra's validate:"max=1000" on the message. Past it
// the face answers a validation error that moyu could have caught itself.
const CreatorMessageMaxRunes = 1000

func creatorKind(status, code int) upstream.Kind {
	switch code {
	case CreatorAlreadyHas, CreatorAppPending, CreatorAppCooldown, AccountBanned:
		return upstream.Rejected
	}
	return clientKind(status, code)
}

// CreatorRefusal answers the house code of a refusal the reader can act on, or
// 0 when the failure is moyu's or OAuth's.
func CreatorRefusal(err error) int {
	e, ok := upstream.As(err)
	if !ok || e.Kind != upstream.Rejected {
		return 0
	}
	code, _ := strconv.Atoi(e.Code)
	return code
}

func (c *Client) CreateCreatorApplication(ctx context.Context, token, source string, evidence json.RawMessage, message string) (*CreatorApplication, error) {
	payload, _ := json.Marshal(map[string]any{"source": source, "evidence": evidence, "message": message})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/creator/applications", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	var app *CreatorApplication
	if err := c.do(req, "creator apply", creatorKind, &app); err != nil {
		return nil, err
	}
	return app, nil
}

func (c *Client) GetMyCreatorApplication(ctx context.Context, token string) (*CreatorApplication, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/creator/applications/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	var app *CreatorApplication
	if err := c.do(req, "creator application", creatorKind, &app); err != nil {
		return nil, err
	}
	return app, nil
}
