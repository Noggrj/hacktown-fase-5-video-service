package config_test

import (
	"testing"

	"github.com/noggrj/fiapx-video-service/internal/platform/config"
)

func TestLoad_DefaultsWhenEnvUnset(t *testing.T) {
	for _, k := range []string{"PORT", "APP_ENV", "SERVICE_NAME", "DB_URL", "JWT_SECRET", "REDIS_ADDR", "S3_BUCKET", "AWS_REGION", "KAFKA_BROKERS"} {
		t.Setenv(k, "")
	}
	cfg := config.Load()
	if cfg.Port != "8080" || cfg.Env != "development" || cfg.ServiceName != "fiapx-video-service" || cfg.AWSRegion != "us-east-1" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.KafkaBrokers != nil {
		t.Fatalf("expected nil brokers when unset, got %v", cfg.KafkaBrokers)
	}
}

func TestLoad_ParsesKafkaBrokersCSV(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "broker-1:9092, broker-2:9092,broker-3:9092")
	cfg := config.Load()
	want := []string{"broker-1:9092", "broker-2:9092", "broker-3:9092"}
	if len(cfg.KafkaBrokers) != len(want) {
		t.Fatalf("expected %v, got %v", want, cfg.KafkaBrokers)
	}
	for i, b := range want {
		if cfg.KafkaBrokers[i] != b {
			t.Fatalf("expected %v, got %v", want, cfg.KafkaBrokers)
		}
	}
}
