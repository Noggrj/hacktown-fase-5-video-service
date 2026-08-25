package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	kafkago "github.com/segmentio/kafka-go"

	events "github.com/noggrj/hacktown-fase-5-events"
	eventskafka "github.com/noggrj/hacktown-fase-5-events/transport/kafka"

	videohttp "github.com/noggrj/hacktown-fase-5-video-service/internal/video/delivery/http"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/gateway"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/usecase"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/cache"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/config"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/db"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/health"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/httpauth"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/idempotency"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/jwt"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/logging"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/messaging"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/metrics"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/storage"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/saga"
)

var version = "dev"

func main() {
	// CI smoke test runs `./server --version` against a container with no
	// DB/Kafka/S3 reachable — must exit immediately instead of falling
	// through to ListenAndServe, which blocks forever.
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}

	cfg := config.Load()
	log := logging.New(cfg.ServiceName)
	slog.SetDefault(log)

	log.Info("starting video-service",
		slog.String("version", version), slog.String("env", cfg.Env), slog.String("port", cfg.Port))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.JWTSecret == "" {
		log.Error("JWT_SECRET is required")
		os.Exit(1)
	}
	verifier, err := jwt.NewVerifier(cfg.JWTSecret)
	if err != nil {
		log.Error("invalid jwt secret", slog.Any("error", err))
		os.Exit(1)
	}

	// ── Postgres ────────────────────────────────────────────────
	pool, err := db.NewPool(ctx, cfg.DBURL)
	if err != nil {
		log.Warn("postgres unavailable at startup — /ready will be degraded", slog.Any("error", err))
	} else {
		log.Info("postgres connected")
		defer pool.Close()
	}

	// ── Redis ───────────────────────────────────────────────────
	redisClient, err := cache.NewClient(cfg.RedisAddr)
	if err != nil {
		log.Warn("redis unavailable at startup — status listing falls back to Postgres only", slog.Any("error", err))
	} else {
		log.Info("redis connected")
		defer func() { _ = redisClient.Close() }()
	}

	// ── S3 ──────────────────────────────────────────────────────
	s3Store, err := storage.NewS3(ctx, cfg.S3Bucket, cfg.AWSRegion, cfg.S3Endpoint, cfg.S3PresignEndpoint, cfg.S3AccessKey, cfg.S3SecretKey)
	if err != nil {
		log.Warn("s3 unavailable at startup — upload/download will fail until reachable", slog.Any("error", err))
	} else {
		log.Info("s3 client ready", slog.String("bucket", cfg.S3Bucket))
	}

	// ── Kafka ───────────────────────────────────────────────────
	var kafkaWriter *kafkago.Writer
	if len(cfg.KafkaBrokers) > 0 {
		kafkaWriter = messaging.NewWriter(cfg.KafkaBrokers)
		defer func() { _ = kafkaWriter.Close() }()
	} else {
		log.Warn("KAFKA_BROKERS unset — publishing and consuming are disabled")
	}

	// ── Repositories / cache ────────────────────────────────────
	var videoRepo *gateway.PostgresVideoRepository
	if pool != nil {
		videoRepo = gateway.NewPostgresVideoRepository(pool.Pool)
	}
	var statusCache usecase.Cache = gateway.NewRedisCache(redisClient) // safe even if redisClient is nil: go-redis calls just error out

	// ── Publisher ───────────────────────────────────────────────
	var pub *saga.Publisher
	if kafkaWriter != nil {
		innerPub, err := eventskafka.NewPublisher(kafkaWriter)
		if err != nil {
			log.Error("failed to init kafka publisher", slog.Any("error", err))
		} else if p, err := saga.NewPublisher(innerPub); err == nil {
			pub = p
		}
	}

	// ── Use cases ──────────────────────────────────────────────
	var (
		uploadUC *usecase.UploadVideoUseCase
		listUC   *usecase.ListVideosUseCase
		getUC    *usecase.GetVideoUseCase
		resultUC *usecase.HandleVideoResultUseCase
	)
	if videoRepo != nil && s3Store != nil && pub != nil {
		uploadUC = usecase.NewUploadVideo(videoRepo, s3Store, statusCache, pub, log)
	}
	if videoRepo != nil {
		listUC = usecase.NewListVideos(videoRepo, statusCache, log)
		resultUC = usecase.NewHandleVideoResult(videoRepo, statusCache, log)
	}
	if videoRepo != nil && s3Store != nil {
		getUC = usecase.NewGetVideo(videoRepo, s3Store)
	}

	// ── Saga consumers (video.processed / video.failed) ─────────
	if len(cfg.KafkaBrokers) > 0 && pool != nil && resultUC != nil {
		idemStore := idempotency.NewPostgresStore(pool.Pool)
		startResultConsumer(ctx, cfg, idemStore, resultUC, log, events.TopicVideoProcessed, "video-service.video-processed")
		startResultConsumer(ctx, cfg, idemStore, resultUC, log, events.TopicVideoFailed, "video-service.video-failed")
	}

	// ── Health probes ──────────────────────────────────────────
	probes := map[string]health.Probe{
		"self": func() health.Check { return health.Check{Status: "healthy"} },
		"postgres": func() health.Check {
			if pool == nil {
				return health.Check{Status: "unhealthy", Detail: "pool not initialized"}
			}
			if err := pool.Healthy(ctx); err != nil {
				return health.Check{Status: "unhealthy", Detail: err.Error()}
			}
			return health.Check{Status: "healthy"}
		},
		"redis": func() health.Check {
			if err := cache.Healthy(ctx, redisClient); err != nil {
				return health.Check{Status: "unhealthy", Detail: err.Error()}
			}
			return health.Check{Status: "healthy"}
		},
		"s3": func() health.Check {
			if s3Store == nil {
				return health.Check{Status: "unhealthy", Detail: "client not initialized"}
			}
			if err := s3Store.Healthy(ctx); err != nil {
				return health.Check{Status: "unhealthy", Detail: err.Error()}
			}
			return health.Check{Status: "healthy"}
		},
	}
	hh := health.New(version, probes)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer, metrics.Middleware)
	r.Get("/health", hh.Live)
	r.Get("/ready", hh.Ready)
	r.Handle("/metrics", metrics.Handler())

	if uploadUC != nil && listUC != nil && getUC != nil {
		r.Group(func(protected chi.Router) {
			protected.Use(httpauth.Middleware(verifier))
			videohttp.NewHandler(uploadUC, listUC, getUC, log).Register(protected)
		})
	}

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r, ReadHeaderTimeout: 5 * time.Second}
	idle := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		shutdownCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("graceful shutdown failed", slog.Any("error", err))
		}
		close(idle)
	}()
	log.Info("server listening", slog.String("port", cfg.Port))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server error", slog.Any("error", err))
		os.Exit(1)
	}
	<-idle
	log.Info("shutdown complete")
}

func startResultConsumer(
	ctx context.Context,
	cfg *config.Config,
	idemStore *idempotency.PostgresStore,
	resultUC *usecase.HandleVideoResultUseCase,
	log *slog.Logger,
	topic events.Topic,
	consumerGroup string,
) {
	reader := messaging.NewReader(cfg.KafkaBrokers, consumerGroup, string(topic))
	dlqWriter := messaging.NewWriter(cfg.KafkaBrokers)
	consumer := saga.NewResultConsumer(reader, dlqWriter, consumerGroup, idemStore, resultUC)
	go func() {
		defer func() { _ = reader.Close(); _ = dlqWriter.Close() }()
		if err := consumer.Start(ctx, log); err != nil && err != context.Canceled {
			log.Error("saga consumer exited", slog.String("topic", string(topic)), slog.Any("error", err))
		}
	}()
}
