package main

import (
	"context"
	"fmt"
	"os"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/storage"
)

func main() {
	ctx := context.Background()
	s3, err := storage.NewS3(ctx, "fiapx-videos-dev", "us-east-1",
		"http://localhost:9000", "http://localhost:9000",
		"minioadmin", "minioadmin")
	if err != nil {
		fmt.Println("NewS3 error:", err)
		os.Exit(1)
	}
	url, err := s3.PresignGet(ctx, "processed/440a39ac-afa7-4c89-9cd3-1ae7758118ec.zip", 900_000_000_000)
	if err != nil {
		fmt.Println("PresignGet error:", err)
		os.Exit(1)
	}
	fmt.Println(url)
}
