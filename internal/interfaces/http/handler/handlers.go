package handler

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	authapp "github.com/temuin/backend/internal/application/auth"
	claimapp "github.com/temuin/backend/internal/application/claim"
	inboxapp "github.com/temuin/backend/internal/application/inbox"
	matchapp "github.com/temuin/backend/internal/application/matching"
	pickupapp "github.com/temuin/backend/internal/application/pickup"
	reportapp "github.com/temuin/backend/internal/application/report"
	userapp "github.com/temuin/backend/internal/application/user"
	"github.com/temuin/backend/internal/domain/claim"
	"github.com/temuin/backend/internal/domain/report"
	"github.com/temuin/backend/internal/domain/user"
	"github.com/temuin/backend/internal/infrastructure/storage"
	"github.com/temuin/backend/internal/interfaces/http/middleware"
	"github.com/temuin/backend/internal/interfaces/http/response"
	"github.com/temuin/backend/internal/shared/apperror"
)

/* ---------------- Auth ---------------- */

type AuthHandler struct{ svc *authapp.Service }

func NewAuthHandler(s *authapp.Service) *AuthHandler { return &AuthHandler{s} }

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var in authapp.RegisterInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Register(c.UserContext(), in)
	if err != nil {
		return err
	}
	return response.Created(c, "Registrasi berhasil", out)
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var in authapp.LoginInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Login(c.UserContext(), in)
	if err != nil {
		return err
	}
	return response.OK(c, "Login berhasil", out)
}

/* ---------------- User ---------------- */

type UserHandler struct {
	svc     *userapp.Service
	reports *reportapp.Service
}

func NewUserHandler(s *userapp.Service, r *reportapp.Service) *UserHandler { return &UserHandler{s, r} }

func (h *UserHandler) Me(c *fiber.Ctx) error {
	out, err := h.svc.Me(c.UserContext(), middleware.UserID(c))
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

func (h *UserHandler) UpdateMe(c *fiber.Ctx) error {
	var in userapp.UpdateProfileInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.UpdateMe(c.UserContext(), middleware.UserID(c), in)
	if err != nil {
		return err
	}
	return response.OK(c, "Profil diperbarui", out)
}

func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	var in userapp.ChangePasswordInput
	if err := bind(c, &in); err != nil {
		return err
	}
	if err := h.svc.ChangePassword(c.UserContext(), middleware.UserID(c), in); err != nil {
		return err
	}
	return response.OK(c, "Password diperbarui", nil)
}

func (h *UserHandler) MyReports(c *fiber.Ctx) error {
	uid := middleware.UserID(c)
	out, meta, err := h.reports.List(c.UserContext(), listInput(c), &uid)
	if err != nil {
		return err
	}
	return response.Paged(c, "", out, meta)
}

func (h *UserHandler) Public(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Public(c.UserContext(), id)
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

/* ---------------- Report ---------------- */

type ReportHandler struct{ svc *reportapp.Service }

func NewReportHandler(s *reportapp.Service) *ReportHandler { return &ReportHandler{s} }

func listInput(c *fiber.Ctx) reportapp.ListInput {
	return reportapp.ListInput{
		Query: c.Query("q"), Type: c.Query("type"), Category: c.Query("category"), Status: c.Query("status"),
		Page: c.QueryInt("page", 1), Limit: c.QueryInt("limit", 20),
	}
}

func (h *ReportHandler) List(c *fiber.Ctx) error {
	out, meta, err := h.svc.List(c.UserContext(), listInput(c), nil)
	if err != nil {
		return err
	}
	return response.Paged(c, "", out, meta)
}

func (h *ReportHandler) Create(c *fiber.Ctx) error {
	var in reportapp.CreateInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Create(c.UserContext(), middleware.UserID(c), in)
	if err != nil {
		return err
	}
	return response.Created(c, "Laporan dipublikasikan", out)
}

func (h *ReportHandler) Get(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Get(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

func (h *ReportHandler) Update(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	var in reportapp.UpdateInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Update(c.UserContext(), middleware.UserID(c), id, in)
	if err != nil {
		return err
	}
	return response.OK(c, "Laporan diperbarui", out)
}

func (h *ReportHandler) Delete(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Delete(c.UserContext(), middleware.UserID(c), id); err != nil {
		return err
	}
	return response.OK(c, "Laporan dihapus", nil)
}

func (h *ReportHandler) Close(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Close(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, "Laporan ditutup", out)
}

func (h *ReportHandler) UpdateCustody(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	var in reportapp.CustodyInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.UpdateCustody(c.UserContext(), middleware.UserID(c), id, in)
	if err != nil {
		return err
	}
	return response.OK(c, "Status penitipan diperbarui", out)
}

func (h *ReportHandler) ConfirmDeposit(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.ConfirmDeposit(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, "Penitipan dikonfirmasi", out)
}

func (h *ReportHandler) Contact(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Contact(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

type categoryItem struct {
	Code  report.Category `json:"code"`
	Label string          `json:"label"`
}

var categoryLabels = map[report.Category]string{
	report.CategoryElectronics: "Elektronik", report.CategoryWallet: "Dompet", report.CategoryIDCard: "Kartu Identitas",
	report.CategoryKeys: "Kunci", report.CategoryBag: "Tas", report.CategoryClothing: "Pakaian",
	report.CategoryAccessories: "Aksesoris", report.CategoryBooks: "Buku", report.CategoryStationery: "Alat Tulis",
	report.CategoryBottle: "Botol Minum", report.CategoryOther: "Lainnya",
}

// Faculties: UB faculty list for the Sign Up "Fakultas" dropdown (public).
func Faculties(c *fiber.Ctx) error { return response.OK(c, "", user.Faculties) }

func (h *ReportHandler) Categories(c *fiber.Ctx) error {
	out := make([]categoryItem, 0, len(report.Categories))
	for _, k := range report.Categories {
		out = append(out, categoryItem{k, categoryLabels[k]})
	}
	return response.OK(c, "", out)
}

/* ---------------- Claim ---------------- */

type ClaimHandler struct{ svc *claimapp.Service }

func NewClaimHandler(s *claimapp.Service) *ClaimHandler { return &ClaimHandler{s} }

func claimFilter(c *fiber.Ctx) (claim.ListFilter, error) {
	f := claim.ListFilter{Page: c.QueryInt("page", 1), Limit: c.QueryInt("limit", 20)}
	if s := strings.ToUpper(c.Query("status")); s != "" {
		st := claim.Status(s)
		switch st {
		case claim.StatusPending, claim.StatusApproved, claim.StatusRejected, claim.StatusItemUnavailable,
			claim.StatusCompleted, claim.StatusCancelled:
			f.Status = &st
		default:
			return f, apperror.Validation(map[string]string{"status": "status klaim tidak dikenal"})
		}
	}
	return f, nil
}

func (h *ClaimHandler) Submit(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	var in claimapp.SubmitInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Submit(c.UserContext(), middleware.UserID(c), id, in)
	if err != nil {
		return err
	}
	return response.Created(c, "Klaim diajukan", out)
}

func (h *ClaimHandler) ListForReport(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.ListForReport(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

func (h *ClaimHandler) Get(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Get(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

func (h *ClaimHandler) Approve(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Approve(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	if out.Status == claim.StatusItemUnavailable {
		return apperror.Unprocessable("ITEM_UNAVAILABLE", "Barang sudah tidak tersedia; klaim ditandai Barang Tidak Tersedia")
	}
	return response.OK(c, "Klaim disetujui", out)
}

func (h *ClaimHandler) Reject(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	var in claimapp.RejectInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Reject(c.UserContext(), middleware.UserID(c), id, in)
	if err != nil {
		return err
	}
	return response.OK(c, "Klaim ditolak", out)
}

func (h *ClaimHandler) Cancel(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Cancel(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, "Klaim dibatalkan", out)
}

func (h *ClaimHandler) Resubmit(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	var in claimapp.SubmitInput
	if err := bind(c, &in); err != nil {
		return err
	}
	out, err := h.svc.Resubmit(c.UserContext(), middleware.UserID(c), id, in)
	if err != nil {
		return err
	}
	return response.Created(c, "Klaim diajukan ulang", out)
}

func (h *ClaimHandler) Complete(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.CompleteHandover(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, "Serah terima selesai", out)
}

func (h *ClaimHandler) Proof(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Proof(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

func (h *ClaimHandler) Incoming(c *fiber.Ctx) error {
	f, err := claimFilter(c)
	if err != nil {
		return err
	}
	out, meta, err := h.svc.ListIncoming(c.UserContext(), middleware.UserID(c), f)
	if err != nil {
		return err
	}
	return response.Paged(c, "", out, meta)
}

func (h *ClaimHandler) Mine(c *fiber.Ctx) error {
	f, err := claimFilter(c)
	if err != nil {
		return err
	}
	out, meta, err := h.svc.ListMine(c.UserContext(), middleware.UserID(c), f)
	if err != nil {
		return err
	}
	return response.Paged(c, "", out, meta)
}

/* ---------------- Inbox / Notifications ---------------- */

type InboxHandler struct{ svc *inboxapp.Service }

func NewInboxHandler(s *inboxapp.Service) *InboxHandler { return &InboxHandler{s} }

func (h *InboxHandler) List(c *fiber.Ctx) error {
	out, meta, err := h.svc.List(c.UserContext(), middleware.UserID(c), c.QueryBool("unread_only", false),
		c.QueryInt("page", 1), c.QueryInt("limit", 20))
	if err != nil {
		return err
	}
	return response.Paged(c, "", out, meta)
}

func (h *InboxHandler) Badge(c *fiber.Ctx) error {
	out, err := h.svc.Badge(c.UserContext(), middleware.UserID(c))
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

func (h *InboxHandler) MarkRead(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.MarkRead(c.UserContext(), middleware.UserID(c), id); err != nil {
		return err
	}
	return response.OK(c, "Notifikasi ditandai dibaca", nil)
}

func (h *InboxHandler) MarkAllRead(c *fiber.Ctx) error {
	n, err := h.svc.MarkAllRead(c.UserContext(), middleware.UserID(c))
	if err != nil {
		return err
	}
	return response.OK(c, "Semua notifikasi ditandai dibaca", fiber.Map{"updated": n})
}

/* ---------------- Pickup / Map ---------------- */

type PickupHandler struct{ svc *pickupapp.Service }

func NewPickupHandler(s *pickupapp.Service) *PickupHandler { return &PickupHandler{s} }

func (h *PickupHandler) List(c *fiber.Ctx) error {
	out, err := h.svc.List(c.UserContext())
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

func (h *PickupHandler) Availability(c *fiber.Ctx) error {
	out, err := h.svc.Availability(c.UserContext())
	if err != nil {
		return err
	}
	return response.OK(c, "", out)
}

func (h *PickupHandler) Summary(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, meta, err := h.svc.Summary(c.UserContext(), id, c.QueryInt("page", 1), c.QueryInt("limit", 20))
	if err != nil {
		return err
	}
	return response.Paged(c, "", out, meta)
}

/* ---------------- Matching ---------------- */

type MatchingHandler struct{ svc *matchapp.Service }

func NewMatchingHandler(s *matchapp.Service) *MatchingHandler { return &MatchingHandler{s} }

func (h *MatchingHandler) Recommend(c *fiber.Ctx) error {
	id, err := paramUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Recommend(c.UserContext(), middleware.UserID(c), id)
	if err != nil {
		return err
	}
	return response.OK(c, matchDisclaimer, out)
}

const matchDisclaimer = "Rekomendasi, perlu diverifikasi"

/* ---------------- Upload ---------------- */

type UploadHandler struct{ store *storage.LocalStorage }

func NewUploadHandler(s *storage.LocalStorage) *UploadHandler { return &UploadHandler{s} }

var uploadFolders = map[string]bool{"reports": true, "claims": true, "avatars": true}

func (h *UploadHandler) Upload(c *fiber.Ctx) error {
	folder := c.FormValue("purpose", "reports")
	if !uploadFolders[folder] {
		return apperror.Validation(map[string]string{"purpose": "harus salah satu dari: reports claims avatars"})
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return apperror.Validation(map[string]string{"file": "wajib diisi (multipart field 'file')"})
	}
	f, err := fh.Open()
	if err != nil {
		return apperror.Internal(err)
	}
	defer f.Close()
	url, err := h.store.SaveImage(folder, f, fh.Size)
	switch {
	case errors.Is(err, storage.ErrTooLarge):
		return apperror.New(apperror.KindValidation, "FILE_TOO_LARGE", "Ukuran berkas melebihi batas")
	case errors.Is(err, storage.ErrInvalidType):
		return apperror.New(apperror.KindValidation, "INVALID_FILE_TYPE", "Hanya gambar JPEG, PNG, atau WEBP")
	case err != nil:
		return apperror.Internal(err)
	}
	return response.Created(c, "Berkas diunggah", fiber.Map{"url": url, "size": fh.Size, "purpose": folder})
}
