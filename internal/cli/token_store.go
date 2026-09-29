package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kakj-go/Judex/pkg/client"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func lockCredentials(ctx context.Context) (func(), error) {
	path := credentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		unlock, err := tryCredentialLock(file)
		if err == nil {
			return func() { unlock(); file.Close() }, nil
		}
		select {
		case <-ctx.Done():
			file.Close()
			return nil, ctx.Err()
		case <-deadline.C:
			file.Close()
			return nil, fmt.Errorf("credential refresh lock timed out")
		case <-time.After(25 * time.Millisecond):
		}
	}
}
func readCredentialDocument() (map[string]string, error) {
	raw, err := os.ReadFile(credentialsPath())
	if err != nil {
		return nil, err
	}
	doc := map[string]string{}
	err = json.Unmarshal(raw, &doc)
	return doc, err
}
func writeTokenPair(server string, pair client.TokenPair) error {
	doc, err := readCredentialDocument()
	if os.IsNotExist(err) {
		doc = map[string]string{}
	} else if err != nil {
		return err
	}
	key := profileKey()
	doc[key] = pair.RefreshToken
	doc["server:"+key] = server
	doc["access:"+key] = pair.AccessToken
	doc["accessExpiry:"+key] = pair.AccessExpiresAt.UTC().Format(time.RFC3339Nano)
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(credentialsPath()), ".credentials-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, credentialsPath())
}
func SaveTokenPair(server string, pair client.TokenPair) error {
	unlock, err := lockCredentials(context.Background())
	if err != nil {
		return err
	}
	defer unlock()
	return writeTokenPair(server, pair)
}
func storedAccessSource(c *client.Client, server string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		unlock, err := lockCredentials(ctx)
		if err != nil {
			return "", err
		}
		defer unlock()
		doc, err := readCredentialDocument()
		if err != nil {
			return "", err
		}
		key := profileKey()
		if strings.TrimSuffix(doc["server:"+key], "/") != strings.TrimSuffix(server, "/") {
			return "", fmt.Errorf("credential belongs to another server")
		}
		expiry, _ := time.Parse(time.RFC3339Nano, doc["accessExpiry:"+key])
		if token := doc["access:"+key]; token != "" && time.Now().Add(30*time.Second).Before(expiry) {
			return token, nil
		}
		if doc[key] == "" {
			return "", fmt.Errorf("sign in required")
		}
		pair, err := c.ExchangeRefresh(ctx, doc[key])
		if err != nil {
			return "", err
		}
		if err = writeTokenPair(server, pair); err != nil {
			return "", err
		}
		return pair.AccessToken, nil
	}
}
