package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kakj-go/Judex/internal/accountimport"
	"os"
)

func main() {
	source, target := os.Getenv("JUDEX_IMPORT_SOURCE_URL"), os.Getenv("JUDEX_IMPORT_TARGET_URL")
	if source == "" || target == "" {
		fmt.Fprintln(os.Stderr, "set JUDEX_IMPORT_SOURCE_URL and JUDEX_IMPORT_TARGET_URL; freeze V1 writes before import")
		os.Exit(1)
	}
	report, err := accountimport.Import(context.Background(), source, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "account import:", err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(report)
}
