package userclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type DeletedUser struct {
	ID        uint      `json:"id"`
	UUID      string    `json:"uuid"`
	DeletedAt time.Time `json:"deleted_at"`
}

type DeletedPage struct {
	Users      []DeletedUser `json:"users"`
	NextCursor string        `json:"next_cursor"`
}

func (c *Client) DeletedUsers(ctx context.Context, cursor string, limit int) (DeletedPage, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/users/deleted?"+q.Encode(), nil)
	if err != nil {
		return DeletedPage{}, err
	}
	req.Header.Set("Authorization", c.authHeader)

	var page DeletedPage
	if err := c.do(req, "users/deleted", clientKind, &page); err != nil {
		return DeletedPage{}, err
	}
	return page, nil
}
