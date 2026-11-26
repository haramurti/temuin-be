package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	authapp "github.com/temuin/backend/internal/application/auth"
	claimapp "github.com/temuin/backend/internal/application/claim"
	"github.com/temuin/backend/internal/application/events"
	helpapp "github.com/temuin/backend/internal/application/help"
	inboxapp "github.com/temuin/backend/internal/application/inbox"
	matchapp "github.com/temuin/backend/internal/application/matching"
	pickupapp "github.com/temuin/backend/internal/application/pickup"
	reportapp "github.com/temuin/backend/internal/application/report"
	userapp "github.com/temuin/backend/internal/application/user"
	"github.com/temuin/backend/internal/config"
	"github.com/temuin/backend/internal/infrastructure/ai"
	"github.com/temuin/backend/internal/infrastructure/persistence/postgres"
	"github.com/temuin/backend/internal/infrastructure/security"
	"github.com/temuin/backend/internal/infrastructure/storage"
	httpapi "github.com/temuin/backend/internal/interfaces/http"
	"github.com/temuin/backend/internal/interfaces/http/handler"
	"github.com/temuin/backend/internal/interfaces/http/response"
)

func main() {
	cfg := config.Load()

	db, err := postgres.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if cfg.AutoMigrate {
		if err := postgres.Migrate(db); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	}
	if cfg.SeedData {
		if err := postgres.SeedPickupPoints(db); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	// infrastructure
	txm := postgres.NewTxManager(db)
	userRepo := postgres.NewUserRepo(db)
	reportRepo := postgres.NewReportRepo(db)
	claimRepo := postgres.NewClaimRepo(db)
	pickupRepo := postgres.NewPickupRepo(db)
	notifRepo := postgres.NewNotificationRepo(db)
	hasher := security.BcryptHasher{}
	tokens := security.NewJWTIssuer(cfg.JWTSecret, cfg.JWTTTL)
	store, err := storage.NewLocal(cfg.UploadDir, cfg.BaseURL, cfg.MaxUploadMB)
	if err != nil {
		log.Fatal(err)
	}
	deepSeek := ai.NewDeepSeek(cfg.SinarCloudAPIKey, cfg.DeepSeekModel, cfg.UploadDir)
	if !deepSeek.Enabled() {
		log.Println("SINARCLOUD_API_KEY not set: matching uses rule-based fallback; help chat uses FAQ fallback")
	}

	// application
	notifier := events.NewNotifier(notifRepo)
	authSvc := authapp.NewService(userRepo, hasher, tokens)
	userSvc := userapp.NewService(userRepo, hasher)
	reportSvc := reportapp.NewService(txm, reportRepo, userRepo, pickupRepo, claimRepo, notifier)
	claimSvc := claimapp.NewService(txm, claimRepo, reportRepo, userRepo, pickupRepo, notifRepo, notifier)
	inboxSvc := inboxapp.NewService(notifRepo, claimRepo)
	pickupSvc := pickupapp.NewService(pickupRepo, reportRepo, reportSvc)
	matchSvc := matchapp.NewService(reportRepo, deepSeek, reportSvc)
	helpSvc := helpapp.NewService(deepSeek, helpapp.NewReportSearcher(reportSvc))

	// interfaces
	app := fiber.New(fiber.Config{
		AppName:      "Temuin API",
		ErrorHandler: response.ErrorHandler,
		BodyLimit:    (cfg.MaxUploadMB + 1) << 20,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	})
	app.Use(recover.New(), requestid.New(), logger.New(), cors.New(cors.Config{
		AllowOrigins: cfg.CORSOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))
	app.Static("/uploads", cfg.UploadDir)

	httpapi.Register(app, httpapi.Handlers{
		Auth:     handler.NewAuthHandler(authSvc),
		User:     handler.NewUserHandler(userSvc, reportSvc),
		Report:   handler.NewReportHandler(reportSvc),
		Claim:    handler.NewClaimHandler(claimSvc),
		Inbox:    handler.NewInboxHandler(inboxSvc),
		Pickup:   handler.NewPickupHandler(pickupSvc),
		Matching: handler.NewMatchingHandler(matchSvc),
		Help:     handler.NewHelpHandler(helpSvc),
		Upload:   handler.NewUploadHandler(store),
	}, tokens)

	go func() {
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			log.Fatal(err)
		}
	}()
	log.Printf("Temuin API listening on :%s", cfg.AppPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(ctx)
}
