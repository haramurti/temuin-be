package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/temuin/backend/internal/domain/claim"
	"github.com/temuin/backend/internal/domain/notification"
	"github.com/temuin/backend/internal/domain/pickup"
	"github.com/temuin/backend/internal/domain/report"
	"github.com/temuin/backend/internal/domain/user"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrUniqueViolation is returned when a partial unique index rejects a write.
var ErrUniqueViolation = errors.New("unique constraint violation")

func mapErr(err error, notFound error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrUniqueViolation
	}
	return err
}

/* ---------- User ---------- */

type UserRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db} }

func (r *UserRepo) Create(ctx context.Context, u *user.User) error {
	err := mapErr(conn(ctx, r.db).Create(u).Error, user.ErrNotFound)
	if errors.Is(err, ErrUniqueViolation) {
		return user.ErrEmailTaken
	}
	return err
}
func (r *UserRepo) Update(ctx context.Context, u *user.User) error {
	err := mapErr(conn(ctx, r.db).Save(u).Error, user.ErrNotFound)
	if errors.Is(err, ErrUniqueViolation) {
		return user.ErrEmailTaken
	}
	return err
}
func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	var u user.User
	return &u, mapErr(conn(ctx, r.db).First(&u, "id = ?", id).Error, user.ErrNotFound)
}
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	var u user.User
	return &u, mapErr(conn(ctx, r.db).First(&u, "lower(email) = lower(?)", email).Error, user.ErrNotFound)
}
func (r *UserRepo) FindByNIM(ctx context.Context, nim string) (*user.User, error) {
	var u user.User
	return &u, mapErr(conn(ctx, r.db).First(&u, "nim = ?", nim).Error, user.ErrNotFound)
}
func (r *UserRepo) FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*user.User, error) {
	out := map[uuid.UUID]*user.User{}
	if len(ids) == 0 {
		return out, nil
	}
	var us []user.User
	if err := conn(ctx, r.db).Where("id IN ?", ids).Find(&us).Error; err != nil {
		return nil, err
	}
	for i := range us {
		out[us[i].ID] = &us[i]
	}
	return out, nil
}

/* ---------- Pickup ---------- */

type PickupRepo struct{ db *gorm.DB }

func NewPickupRepo(db *gorm.DB) *PickupRepo { return &PickupRepo{db} }

func (r *PickupRepo) List(ctx context.Context, activeOnly bool) ([]pickup.PickupPoint, error) {
	var ps []pickup.PickupPoint
	q := conn(ctx, r.db).Order("name ASC")
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}
	return ps, q.Find(&ps).Error
}
func (r *PickupRepo) FindByID(ctx context.Context, id uuid.UUID) (*pickup.PickupPoint, error) {
	var p pickup.PickupPoint
	return &p, mapErr(conn(ctx, r.db).First(&p, "id = ?", id).Error, pickup.ErrNotFound)
}
func (r *PickupRepo) FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*pickup.PickupPoint, error) {
	out := map[uuid.UUID]*pickup.PickupPoint{}
	if len(ids) == 0 {
		return out, nil
	}
	var ps []pickup.PickupPoint
	if err := conn(ctx, r.db).Where("id IN ?", ids).Find(&ps).Error; err != nil {
		return nil, err
	}
	for i := range ps {
		out[ps[i].ID] = &ps[i]
	}
	return out, nil
}

/* ---------- Report ---------- */

type ReportRepo struct{ db *gorm.DB }

func NewReportRepo(db *gorm.DB) *ReportRepo { return &ReportRepo{db} }

func (r *ReportRepo) Create(ctx context.Context, rp *report.Report) error {
	return conn(ctx, r.db).Create(rp).Error
}
func (r *ReportRepo) Update(ctx context.Context, rp *report.Report) error {
	return conn(ctx, r.db).Save(rp).Error
}
func (r *ReportRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return conn(ctx, r.db).Delete(&report.Report{}, "id = ?", id).Error
}
func (r *ReportRepo) FindByID(ctx context.Context, id uuid.UUID) (*report.Report, error) {
	var rp report.Report
	return &rp, mapErr(conn(ctx, r.db).First(&rp, "id = ?", id).Error, report.ErrNotFound)
}
func (r *ReportRepo) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*report.Report, error) {
	var rp report.Report
	err := conn(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).First(&rp, "id = ?", id).Error
	return &rp, mapErr(err, report.ErrNotFound)
}
func (r *ReportRepo) FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*report.Report, error) {
	out := map[uuid.UUID]*report.Report{}
	if len(ids) == 0 {
		return out, nil
	}
	var rs []report.Report
	if err := conn(ctx, r.db).Where("id IN ?", ids).Find(&rs).Error; err != nil {
		return nil, err
	}
	for i := range rs {
		out[rs[i].ID] = &rs[i]
	}
	return out, nil
}

func (r *ReportRepo) List(ctx context.Context, f report.Filter) ([]report.Report, int64, error) {
	q := conn(ctx, r.db).Model(&report.Report{})
	if s := strings.TrimSpace(f.Query); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("(lower(title) LIKE ? OR lower(description) LIKE ? OR lower(location_name) LIKE ? OR lower(coalesce(color,'')) LIKE ?)",
			like, like, like, like)
	}
	if f.Type != nil {
		q = q.Where("type = ?", *f.Type)
	}
	if f.Category != nil {
		q = q.Where("category = ?", *f.Category)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	if f.Custody != nil {
		q = q.Where("custody = ?", *f.Custody)
	}
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.PickupPointID != nil {
		q = q.Where("pickup_point_id = ?", *f.PickupPointID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	off, lim := paginate(f.Page, f.Limit)
	var rs []report.Report
	err := q.Order("created_at DESC").Offset(off).Limit(lim).Find(&rs).Error
	return rs, total, err
}

func (r *ReportRepo) FindCandidates(ctx context.Context, cq report.CandidateQuery) ([]report.Report, error) {
	var rs []report.Report
	err := conn(ctx, r.db).
		Where("type = ? AND category = ? AND status = ? AND user_id <> ?", cq.Type, cq.Category, report.StatusActive, cq.ExcludeUserID).
		Where("occurred_at BETWEEN ? AND ?", cq.From, cq.To).
		Where("custody IS NULL OR custody <> ?", report.CustodyUnavailable).
		Order("created_at DESC").Limit(200).Find(&rs).Error
	return rs, err
}

func (r *ReportRepo) CountDepositedByPickupPoint(ctx context.Context) (map[uuid.UUID]int64, error) {
	type row struct {
		PickupPointID uuid.UUID
		Total         int64
	}
	var rows []row
	err := conn(ctx, r.db).Model(&report.Report{}).
		Select("pickup_point_id, COUNT(*) AS total").
		Where("type = ? AND status = ? AND custody = ? AND pickup_point_id IS NOT NULL",
			report.TypeFound, report.StatusActive, report.CustodyDeposited).
		Group("pickup_point_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, x := range rows {
		out[x.PickupPointID] = x.Total
	}
	return out, nil
}

/* ---------- Claim ---------- */

type ClaimRepo struct{ db *gorm.DB }

func NewClaimRepo(db *gorm.DB) *ClaimRepo { return &ClaimRepo{db} }

func claimErr(err error) error {
	err = mapErr(err, claim.ErrNotFound)
	if errors.Is(err, ErrUniqueViolation) {
		return claim.ErrConflict
	}
	return err
}
func (r *ClaimRepo) Create(ctx context.Context, c *claim.Claim) error {
	return claimErr(conn(ctx, r.db).Create(c).Error)
}
func (r *ClaimRepo) Update(ctx context.Context, c *claim.Claim) error {
	return claimErr(conn(ctx, r.db).Save(c).Error)
}
func (r *ClaimRepo) FindByID(ctx context.Context, id uuid.UUID) (*claim.Claim, error) {
	var c claim.Claim
	return &c, mapErr(conn(ctx, r.db).First(&c, "id = ?", id).Error, claim.ErrNotFound)
}
func (r *ClaimRepo) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*claim.Claim, error) {
	var c claim.Claim
	err := conn(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).First(&c, "id = ?", id).Error
	return &c, mapErr(err, claim.ErrNotFound)
}
func (r *ClaimRepo) list(ctx context.Context, col string, id uuid.UUID, f claim.ListFilter) ([]claim.Claim, int64, error) {
	q := conn(ctx, r.db).Model(&claim.Claim{}).Where(col+" = ?", id)
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	off, lim := paginate(f.Page, f.Limit)
	var cs []claim.Claim
	// pending first (needs action), then newest
	err := q.Order("CASE WHEN status = 'PENDING' THEN 0 ELSE 1 END, updated_at DESC").Offset(off).Limit(lim).Find(&cs).Error
	return cs, total, err
}
func (r *ClaimRepo) ListByFinder(ctx context.Context, id uuid.UUID, f claim.ListFilter) ([]claim.Claim, int64, error) {
	return r.list(ctx, "finder_id", id, f)
}
func (r *ClaimRepo) ListByClaimant(ctx context.Context, id uuid.UUID, f claim.ListFilter) ([]claim.Claim, int64, error) {
	return r.list(ctx, "claimant_id", id, f)
}
func (r *ClaimRepo) ListByReport(ctx context.Context, reportID uuid.UUID) ([]claim.Claim, error) {
	var cs []claim.Claim
	return cs, conn(ctx, r.db).Where("report_id = ?", reportID).Order("created_at ASC").Find(&cs).Error
}
func (r *ClaimRepo) ExistsBetween(ctx context.Context, reportID, claimantID uuid.UUID, st []claim.Status) (bool, error) {
	var n int64
	err := conn(ctx, r.db).Model(&claim.Claim{}).
		Where("report_id = ? AND claimant_id = ? AND status IN ?", reportID, claimantID, st).Count(&n).Error
	return n > 0, err
}
func (r *ClaimRepo) HasOpenClaim(ctx context.Context, reportID, claimantID uuid.UUID) (bool, error) {
	return r.ExistsBetween(ctx, reportID, claimantID, []claim.Status{claim.StatusPending, claim.StatusApproved})
}
func (r *ClaimRepo) HasApproved(ctx context.Context, reportID uuid.UUID) (bool, error) {
	var n int64
	err := conn(ctx, r.db).Model(&claim.Claim{}).
		Where("report_id = ? AND status IN ?", reportID, []claim.Status{claim.StatusApproved, claim.StatusCompleted}).Count(&n).Error
	return n > 0, err
}
func (r *ClaimRepo) ExistsForReport(ctx context.Context, reportID uuid.UUID) (bool, error) {
	var n int64
	err := conn(ctx, r.db).Model(&claim.Claim{}).Where("report_id = ?", reportID).Count(&n).Error
	return n > 0, err
}

/* ---------- Notification ---------- */

type NotificationRepo struct{ db *gorm.DB }

func NewNotificationRepo(db *gorm.DB) *NotificationRepo { return &NotificationRepo{db} }

func (r *NotificationRepo) CreateIfAbsent(ctx context.Context, n *notification.Notification) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return conn(ctx, r.db).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_key"}}, DoNothing: true}).Create(n).Error
}
func (r *NotificationRepo) List(ctx context.Context, rcp uuid.UUID, unreadOnly bool, page, limit int) ([]notification.Notification, int64, error) {
	q := conn(ctx, r.db).Model(&notification.Notification{}).Where("recipient_id = ?", rcp)
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	off, lim := paginate(page, limit)
	var ns []notification.Notification
	return ns, total, q.Order("created_at DESC").Offset(off).Limit(lim).Find(&ns).Error
}
func (r *NotificationRepo) CountUnread(ctx context.Context, rcp uuid.UUID) (int64, error) {
	var n int64
	return n, conn(ctx, r.db).Model(&notification.Notification{}).Where("recipient_id = ? AND read_at IS NULL", rcp).Count(&n).Error
}
func (r *NotificationRepo) MarkRead(ctx context.Context, id, rcp uuid.UUID, at time.Time) error {
	res := conn(ctx, r.db).Model(&notification.Notification{}).Where("id = ? AND recipient_id = ?", id, rcp).
		Where("read_at IS NULL").Update("read_at", at)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var n int64
		conn(ctx, r.db).Model(&notification.Notification{}).Where("id = ? AND recipient_id = ?", id, rcp).Count(&n)
		if n == 0 {
			return notification.ErrNotFound
		}
	}
	return nil
}
func (r *NotificationRepo) MarkAllRead(ctx context.Context, rcp uuid.UUID, at time.Time) (int64, error) {
	res := conn(ctx, r.db).Model(&notification.Notification{}).Where("recipient_id = ? AND read_at IS NULL", rcp).Update("read_at", at)
	return res.RowsAffected, res.Error
}
func (r *NotificationRepo) MarkReadByClaim(ctx context.Context, claimID, rcp uuid.UUID, at time.Time) error {
	return conn(ctx, r.db).Model(&notification.Notification{}).
		Where("claim_id = ? AND recipient_id = ? AND read_at IS NULL", claimID, rcp).Update("read_at", at).Error
}
