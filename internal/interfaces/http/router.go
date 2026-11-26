package http

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/temuin/backend/internal/domain/user"
	"github.com/temuin/backend/internal/interfaces/http/handler"
	"github.com/temuin/backend/internal/interfaces/http/middleware"
	"github.com/temuin/backend/internal/shared/apperror"
)

type Handlers struct {
	Auth     *handler.AuthHandler
	User     *handler.UserHandler
	Report   *handler.ReportHandler
	Claim    *handler.ClaimHandler
	Inbox    *handler.InboxHandler
	Pickup   *handler.PickupHandler
	Matching *handler.MatchingHandler
	Help     *handler.HelpHandler
	Upload   *handler.UploadHandler
}

func Register(app *fiber.App, h Handlers, tokens user.TokenIssuer) {
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "time": time.Now().UTC()})
	})
	app.Get("/docs/openapi.yaml", func(c *fiber.Ctx) error { return c.SendFile("./docs/openapi.yaml") })

	v1 := app.Group("/api/v1")

	authLimit := limiter.New(limiter.Config{Max: 20, Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return apperror.New(apperror.KindTooManyRequests, "RATE_LIMITED", "Terlalu banyak percobaan, coba lagi nanti")
		}})
	a := v1.Group("/auth", authLimit)
	a.Post("/register", h.Auth.Register)
	a.Post("/login", h.Auth.Login)
	v1.Get("/faculties", handler.Faculties)

	p := v1.Group("", middleware.Auth(tokens))
	helpLimit := limiter.New(limiter.Config{Max: 20, Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return middleware.UserID(c).String() },
		LimitReached: func(c *fiber.Ctx) error {
			return apperror.New(apperror.KindTooManyRequests, "RATE_LIMITED", "Terlalu banyak pesan bantuan, coba lagi sebentar lagi")
		}})
	p.Get("/help/topics", h.Help.Topics)
	p.Post("/help/chat", helpLimit, h.Help.Chat)

	// Users / profile
	p.Get("/users/me", h.User.Me)
	p.Patch("/users/me", h.User.UpdateMe)
	p.Put("/users/me/password", h.User.ChangePassword)
	p.Get("/users/me/reports", h.User.MyReports)
	p.Get("/users/:id", h.User.Public)

	// Uploads
	p.Post("/uploads", h.Upload.Upload)

	// Reports (feed, CRUD, custody)
	p.Get("/categories", h.Report.Categories)
	p.Get("/reports", h.Report.List)
	p.Post("/reports", h.Report.Create)
	p.Get("/reports/:id", h.Report.Get)
	p.Patch("/reports/:id", h.Report.Update)
	p.Delete("/reports/:id", h.Report.Delete)
	p.Post("/reports/:id/close", h.Report.Close)
	p.Patch("/reports/:id/custody", h.Report.UpdateCustody)
	p.Post("/reports/:id/custody/confirm-deposit", h.Report.ConfirmDeposit)
	p.Get("/reports/:id/contact", h.Report.Contact)

	// Matching (AI) — per-user rate limit (PRD: membatasi jumlah permintaan per pengguna)
	matchLimit := limiter.New(limiter.Config{Max: 10, Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return middleware.UserID(c).String() },
		LimitReached: func(c *fiber.Ctx) error {
			return apperror.New(apperror.KindTooManyRequests, "RATE_LIMITED", "Terlalu banyak permintaan rekomendasi, coba lagi sebentar lagi")
		}})
	p.Post("/reports/:id/matches", matchLimit, h.Matching.Recommend)

	// Claims
	p.Post("/reports/:id/claims", h.Claim.Submit)
	p.Get("/reports/:id/claims", h.Claim.ListForReport)
	p.Get("/claims/incoming", h.Claim.Incoming)
	p.Get("/claims/mine", h.Claim.Mine)
	p.Get("/claims/:id", h.Claim.Get)
	p.Post("/claims/:id/approve", h.Claim.Approve)
	p.Post("/claims/:id/reject", h.Claim.Reject)
	p.Post("/claims/:id/cancel", h.Claim.Cancel)
	p.Post("/claims/:id/resubmit", h.Claim.Resubmit)
	p.Post("/claims/:id/complete", h.Claim.Complete)
	p.Get("/claims/:id/proof", h.Claim.Proof)

	// Inbox / notifications
	p.Get("/notifications", h.Inbox.List)
	p.Get("/notifications/badge", h.Inbox.Badge)
	p.Patch("/notifications/read-all", h.Inbox.MarkAllRead)
	p.Patch("/notifications/:id/read", h.Inbox.MarkRead)

	// Pickup points / map
	p.Get("/pickup-points", h.Pickup.List)
	p.Get("/pickup-points/:id/items", h.Pickup.Summary)
	p.Get("/map/availability", h.Pickup.Availability)
}
