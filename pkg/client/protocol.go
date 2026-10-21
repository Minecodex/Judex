package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type protocolCheck struct {
	once sync.Once
	err  error
}

func (c *Client) CheckProtocol(ctx context.Context) error {
	c.protocol.once.Do(func() {
		req, err := http.NewRequestWithContext(ctx, "GET", c.Server+"/api/v1/system", nil)
		if err != nil {
			c.protocol.err = err
			return
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			c.protocol.err = err
			return
		}
		defer resp.Body.Close()
		var data struct {
			Data struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
		}
		if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data); err != nil {
			c.protocol.err = err
			return
		}
		if resp.StatusCode != 200 || data.Data.ProtocolVersion != "2" {
			c.protocol.err = &CLIError{Status: 400, Code: "UNSUPPORTED_SCHEMA", Message: fmt.Sprintf("Judex V2 requires protocol 2; server reports %q. Install matching Server/Web/CLI versions.", data.Data.ProtocolVersion)}
		}
	})
	return c.protocol.err
}
func (c *Client) PutBytes(ctx context.Context, path string, body io.Reader, size int64, key string) error {
	if err := c.CheckProtocol(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", c.Server+"/api/v1"+path, body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Idempotency-Key", key)
	if err = c.authorize(ctx, req); err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var env envelope
		_ = json.NewDecoder(resp.Body).Decode(&env)
		if env.Error != nil {
			return &CLIError{Status: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		return &CLIError{Status: resp.StatusCode, Code: "HTTP_ERROR"}
	}
	return nil
}
func (c *Client) Download(ctx context.Context, path string, out io.Writer) error {
	if err := c.CheckProtocol(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.Server+"/api/v1"+path, nil)
	if err != nil {
		return err
	}
	if err = c.authorize(ctx, req); err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &CLIError{Status: resp.StatusCode, Code: "HTTP_ERROR", Message: resp.Status}
	}
	_, err = io.Copy(out, resp.Body)
	return err
}
