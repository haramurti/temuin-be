// Package events records status events (recordStatusEvent, Fungsi 3) as
// idempotent notifications. Used by claim and report use cases.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/temuin/backend/internal/domain/claim"
	"github.com/temuin/backend/internal/domain/notification"
	"github.com/temuin/backend/internal/domain/report"
)

type Notifier struct{ repo notification.Repository }

func NewNotifier(r notification.Repository) *Notifier { return &Notifier{repo: r} }

// ClaimStatusChanged emits the notification(s) matching the claim's current status.
func (n *Notifier) ClaimStatusChanged(ctx context.Context, c *claim.Claim, r *report.Report) error {
	type msg struct {
		to          uuid.UUID
		t           notification.Type
		title, body string
	}
	var ms []msg
	item := r.Title
	switch c.Status {
	case claim.StatusPending:
		ms = append(ms, msg{c.FinderID, notification.TypeClaimSubmitted, "Klaim baru masuk",
			fmt.Sprintf("Ada klaim baru untuk \"%s\". Periksa ciri rahasia sebelum menyetujui.", item)})
	case claim.StatusApproved:
		where := "Atur pertemuan langsung dengan penemu."
		if c.PickupMethod != nil && *c.PickupMethod == claim.PickupAtPoint {
			where = "Ambil barang di pos penitipan dengan menunjukkan bukti persetujuan."
		}
		ms = append(ms, msg{c.ClaimantID, notification.TypeClaimApproved, "Klaim disetujui",
			fmt.Sprintf("Klaim kamu untuk \"%s\" disetujui. %s", item, where)})
	case claim.StatusRejected:
		reason := ""
		if c.RejectionReason != nil {
			reason = " Alasan: " + *c.RejectionReason
		}
		ms = append(ms, msg{c.ClaimantID, notification.TypeClaimRejected, "Klaim ditolak",
			fmt.Sprintf("Klaim kamu untuk \"%s\" ditolak.%s Kamu dapat mengajukan ulang dengan bukti baru.", item, reason)})
	case claim.StatusCancelled:
		ms = append(ms, msg{c.FinderID, notification.TypeClaimCancelled, "Klaim dibatalkan",
			fmt.Sprintf("Pengaju membatalkan klaim untuk \"%s\".", item)})
	case claim.StatusItemUnavailable:
		ms = append(ms, msg{c.ClaimantID, notification.TypeClaimItemUnavailable, "Barang tidak tersedia",
			fmt.Sprintf("Barang \"%s\" sudah tidak tersedia sehingga klaim tidak dapat dilanjutkan.", item)})
	case claim.StatusCompleted:
		ms = append(ms, msg{c.ClaimantID, notification.TypeClaimCompleted, "Serah terima selesai",
			fmt.Sprintf("Serah terima \"%s\" telah dikonfirmasi selesai oleh penemu.", item)})
	}
	for _, m := range ms {
		cid, rid := c.ID, r.ID
		err := n.repo.CreateIfAbsent(ctx, &notification.Notification{
			RecipientID: m.to, Type: m.t, Title: m.title, Body: m.body, ClaimID: &cid, ReportID: &rid,
			// One event per (claim, status, recipient). PENDING may repeat only via a new claim id.
			EventKey: notification.EventKey(m.t, c.ID, m.to, string(c.Status)),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// PickupChanged tells an approved claimant the item moved.
func (n *Notifier) PickupChanged(ctx context.Context, c *claim.Claim, r *report.Report, at time.Time) error {
	cid, rid := c.ID, r.ID
	return n.repo.CreateIfAbsent(ctx, &notification.Notification{
		RecipientID: c.ClaimantID, Type: notification.TypePickupChanged, Title: "Lokasi pengambilan berubah",
		Body:    fmt.Sprintf("Lokasi pengambilan \"%s\" diperbarui. Cek detail klaim untuk petunjuk terbaru.", r.Title),
		ClaimID: &cid, ReportID: &rid,
		EventKey: notification.EventKey(notification.TypePickupChanged, c.ID, c.ClaimantID, fmt.Sprint(at.UnixNano())),
	})
}
