package client

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type TokenPair struct {
	AccessToken      string    `json:"accessToken"`
	AccessExpiresAt  time.Time `json:"accessExpiresAt"`
	RefreshToken     string    `json:"refreshToken"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
	Scopes           []string  `json:"scopes"`
}

func (c *Client) ExchangeRefresh(ctx context.Context, refresh string) (TokenPair, error) {
	var pair TokenPair
	err := c.Do(ctx, "POST", "/auth/token/refresh", map[string]string{"refreshToken": refresh}, &pair, "")
	return pair, err
}
func (c *Client) authorize(ctx context.Context, request *http.Request) error {
	path := strings.TrimPrefix(request.URL.Path, "/api/v1")
	public := path == "/system" || path == "/capabilities" || path == "/auth/register" || path == "/auth/login" || path == "/auth/recover" || path == "/auth/device/authorizations" || path == "/auth/device/token" || path == "/auth/token/refresh"
	if public {
		return nil
	}
	token := c.Token
	if c.TokenSource != nil {
		var err error
		token, err = c.TokenSource(ctx)
		if err != nil {
			return err
		}
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}
