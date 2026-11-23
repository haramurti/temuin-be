package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/temuin/backend/internal/config"
	"github.com/temuin/backend/internal/domain/claim"
	"github.com/temuin/backend/internal/domain/notification"
	"github.com/temuin/backend/internal/domain/pickup"
	"github.com/temuin/backend/internal/domain/report"
	"github.com/temuin/backend/internal/domain/user"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg *config.Config) (*gorm.DB, error) {
	lvl := logger.Warn
	if cfg.AppEnv == "development" {
		lvl = logger.Info
	}
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:  logger.Default.LogMode(lvl),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

// Migrate creates tables plus the partial unique indexes that enforce
// business invariants at the database level.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&user.User{}, &pickup.PickupPoint{}, &report.Report{}, &claim.Claim{}, &notification.Notification{},
	); err != nil {
		return err
	}
	stmts := []string{
		// Only one APPROVED/COMPLETED claim per report (Fungsi 1: satu persetujuan per barang).
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_claims_one_approved_per_report
		   ON claims (report_id) WHERE status IN ('APPROVED','COMPLETED')`,
		// A claimant can have at most one open (PENDING/APPROVED) claim per report (no double submit).
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_claims_one_open_per_claimant
		   ON claims (report_id, claimant_id) WHERE status IN ('PENDING','APPROVED')`,
		`CREATE INDEX IF NOT EXISTS ix_reports_search
		   ON reports USING gin (to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(description,'')))`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// TxManager implements tx.Manager.
type TxManager struct{ db *gorm.DB }

func NewTxManager(db *gorm.DB) *TxManager { return &TxManager{db: db} }

type txKey struct{}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx) // already inside a tx
	}
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// conn returns the tx from ctx when present, else the base db.
func conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return db.WithContext(ctx)
}

func paginate(page, limit int) (offset, lim int) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	return (page - 1) * limit, limit
}
