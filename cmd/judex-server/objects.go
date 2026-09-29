package main

import (
	"context"
	"fmt"
	"github.com/kakj-go/Judex/internal/infrastructure/objectstore"
	"os"
)

func runObjectArchive(operation string) error {
	store, err := objectstore.New(context.Background(), objectstore.Options{
		Endpoint: os.Getenv("JUDEX_S3_ENDPOINT"), Region: os.Getenv("JUDEX_S3_REGION"), Bucket: os.Getenv("JUDEX_S3_BUCKET"),
		AccessKeyID: os.Getenv("JUDEX_S3_ACCESS_KEY"), SecretAccessKey: os.Getenv("JUDEX_S3_SECRET_KEY"), PathStyle: os.Getenv("JUDEX_S3_PATH_STYLE") != "false",
	})
	if err != nil {
		return err
	}
	switch operation {
	case "init":
		return store.InitBucket(context.Background())
	case "export":
		return store.Export(context.Background(), os.Stdout)
	case "import":
		return store.Import(context.Background(), os.Stdin)
	default:
		return fmt.Errorf("usage: judex-server objects init|export|import")
	}
}
