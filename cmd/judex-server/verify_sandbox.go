package main

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/app"
	"github.com/kakj-go/Judex/internal/config"
	"os"
	"time"
)

func verifySandbox(raw string) error {
	group, err := uuid.Parse(raw)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Mode = config.ModeMigrate
	runtime, err := app.New(cfg, nil)
	if err != nil {
		return err
	}
	defer runtime.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	result, err := runtime.VerifyGroupSandbox(ctx, group)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
