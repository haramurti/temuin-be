# Temuin API Contract v1

Base URL lokal: `http://localhost:8080/api/v1` (emulator Android: `http://10.0.2.2:8080/api/v1`)
Base URL publik (ngrok): `https://remissly-crumbier-michelle.ngrok-free.dev/api/v1` — kirim header `ngrok-skip-browser-warning: 1`.
Spesifikasi lengkap (import ke Apidog): [`openapi.yaml`](openapi.yaml) — juga disajikan di `GET /docs/openapi.yaml`.

## 1. Konvensi

| Hal | Aturan |
|---|---|
| Auth | `Authorization: Bearer <access_token>` untuk semua endpoint kecuali `/auth/*`, `/faculties`, `/health`. JWT HS256, default 72 jam. |
| Format | JSON, `snake_case`. Waktu RFC3339 UTC (`2026-11-16T07:30:00Z`). ID = UUID v4. |
| Pagination | `?page=1&limit=20` (maks 100) → `meta: {page, limit, total, total_pages}` |
| Partial update | `PATCH` hanya mengubah field yang dikirim. String kosong `""` mengosongkan field opsional profil. |
| Rate limit | `/auth/*` 20 req/menit/IP · `/reports/{id}/matches` 10 req/menit/user → `429 RATE_LIMITED` |

### Envelope sukses
```json
{ "success": true, "message": "Klaim diajukan", "data": { }, "meta": { } }
```
### Envelope error
```json
{ "success": false, "error": { "code": "VALIDATION_ERROR", "message": "Input tidak valid",
  "details": { "title": "wajib diisi" } } }
```

| HTTP | Kapan | Contoh `code` |
|---|---|---|
| 400 | Body/ID tidak bisa di-parse | `INVALID_BODY`, `INVALID_ID` |
| 401 | Token tidak ada/invalid, login gagal | `UNAUTHENTICATED`, `INVALID_TOKEN`, `INVALID_CREDENTIALS` |
| 403 | Bukan pihak yang berwenang | `SELF_CLAIM`, `NOT_FINDER`, `NOT_CLAIMANT`, `NOT_REPORT_OWNER`, `CONTACT_RESTRICTED` |
| 404 | Tidak ada / tidak boleh tahu keberadaannya | `REPORT_NOT_FOUND`, `CLAIM_NOT_FOUND`, … |
| 409 | Bentrok status / duplikasi | `NIM_TAKEN`, `EMAIL_TAKEN`, `DUPLICATE_CLAIM`, `ALREADY_APPROVED`, `INVALID_CLAIM_STATE`, `REPORT_HAS_CLAIMS` |
| 422 | Validasi field / aturan bisnis | `VALIDATION_ERROR`, `ITEM_UNAVAILABLE`, `REPORT_NOT_ACTIVE`, `INVALID_CUSTODY_STATE`, `PROOF_UNAVAILABLE` |
| 429 | Rate limit | `RATE_LIMITED` |

> `409 INVALID_CLAIM_STATE` = data di layar sudah basi → client wajib **reload** detail klaim (PRD Fungsi 3: aksi lama dibatalkan & status dimuat ulang).

## 2. Daftar Endpoint

| Modul | Method & Path | Siapa | Ringkas |
|---|---|---|---|
| Auth | `POST /auth/register` | publik | Sign Up (nama, NIM, fakultas, WA, sandi + konfirmasi, `agree_terms`), langsung dapat token |
| | `POST /auth/login` | publik | Sign In pakai `identifier` = NIM atau email |
| | `GET /faculties` | publik | Dropdown Fakultas (17 fakultas UB) |
| Users | `GET /users/me` · `PATCH /users/me` | user | Profil & Edit Profil |
| | `PUT /users/me/password` | user | Ganti password |
| | `GET /users/me/reports` | user | Laporan Saya (semua status) |
| | `GET /users/{id}` | user | Profil publik |
| Upload | `POST /uploads` (multipart `file`, `purpose`) | user | Foto laporan/bukti klaim/avatar → `url` |
| Reports | `GET /categories` | user | Enum kategori + label |
| | `GET /reports?q&type&category&status&page&limit` | user | Feed + pencarian + filter. Tab Semua/Barang Hilang/Ditemukan = `type` kosong/`LOST`/`FOUND`; badge "48 Aktif" = `meta.total` dari `status=ACTIVE` |
| | `POST /reports` | user | Buat laporan. Dropdown "Lokasi" = `location_point_id` (pos satpam fakultas dari `/pickup-points`) + `location_name` opsional untuk detail |
| | `GET /reports/{id}` | user | Detail + flag `viewer` |
| | `PATCH /reports/{id}` · `DELETE /reports/{id}` | pemilik | Edit / hapus (hapus hanya jika belum ada klaim) |
| | `POST /reports/{id}/close` | pemilik | Tutup tanpa serah terima |
| | `GET /reports/{id}/contact` | user | Hubungi Pelapor / kontak penemu (terbatas) |
| Custody | `PATCH /reports/{id}/custody` | pemilik FOUND | Pegang sendiri / pilih-ganti pos / tidak tersedia |
| | `POST /reports/{id}/custody/confirm-deposit` | pemilik FOUND | Konfirmasi sudah dititipkan → masuk peta |
| Matching | `POST /reports/{id}/matches` | user | Rekomendasi kecocokan |
| Claims | `POST /reports/{id}/claims` | calon pemilik | Ajukan klaim |
| | `GET /reports/{id}/claims` | pemilik | Semua klaim di laporan saya |
| | `GET /claims/incoming` | penemu | Inbox: klaim masuk |
| | `GET /claims/mine` | calon pemilik | Inbox: status klaim saya |
| | `GET /claims/{id}` | pihak terkait | Detail + `allowed_actions` |
| | `POST /claims/{id}/approve` · `/reject` | penemu | Setujui / tolak (`reason`) |
| | `POST /claims/{id}/cancel` | pengaju | Batalkan saat PENDING |
| | `POST /claims/{id}/resubmit` | pengaju | Ajukan ulang setelah ditolak |
| | `POST /claims/{id}/complete` | penemu | Serah terima selesai |
| | `GET /claims/{id}/proof` | pihak terkait | Bukti persetujuan |
| Inbox | `GET /notifications?unread_only` | user | Daftar notifikasi |
| | `GET /notifications/badge` | user | Angka badge Inbox |
| | `PATCH /notifications/{id}/read` · `/notifications/read-all` | user | Tandai dibaca |
| Map | `GET /pickup-points` | user | Dropdown pos |
| | `GET /map/availability` | user | Marker (hanya pos dengan stok > 0) |
| | `GET /pickup-points/{id}/items` | user | Bottom sheet: barang tersedia di pos |

## 3. State Machine

### Laporan (`report.status`)
```
ACTIVE ──(claim complete)──▶ COMPLETED
ACTIVE ──(owner close)─────▶ CLOSED
```

### Penitipan (`report.custody`, hanya FOUND)
```
HELD_BY_FINDER ◀──────────────┐
     │ PATCH custody=AWAITING_DROPOFF + pickup_point_id
     ▼                        │
AWAITING_DROPOFF ─confirm-deposit─▶ DEPOSITED  (dihitung di peta)
     ▲                        │        │
     └── ganti pos (PATCH) ───┴────────┘
semua ──PATCH custody=UNAVAILABLE──▶ UNAVAILABLE (klaim terbuka → ITEM_UNAVAILABLE)
```

### Klaim (`claim.status`)
```
PENDING ──approve──▶ APPROVED ──complete──▶ COMPLETED
   │ ├──reject──▶ REJECTED ──resubmit──▶ (klaim baru PENDING, previous_claim_id)
   │ └──cancel──▶ CANCELLED
   └─(barang tidak tersedia / klaim lain selesai)─▶ ITEM_UNAVAILABLE   (juga dari APPROVED)
```

### `allowed_actions` (role × status)

| Status | FINDER | CLAIMANT |
|---|---|---|
| PENDING | `APPROVE`, `REJECT` (jika barang tersedia) | `CANCEL` |
| APPROVED | `VIEW_PROOF`, `CONTACT`, `COMPLETE_HANDOVER` | `VIEW_PROOF`, `CONTACT` |
| REJECTED | – | `RESUBMIT` (jika barang tersedia) |
| COMPLETED | `VIEW_PROOF` | `VIEW_PROOF` |
| CANCELLED / ITEM_UNAVAILABLE | – | – |

Client cukup render tombol sesuai array ini.

## 4. Aturan Bisnis Penting

**Fungsi 1 – Klaim**
- Klaim hanya pada laporan `FOUND`, `ACTIVE`, custody ≠ `UNAVAILABLE`.
- Tidak boleh klaim laporan sendiri → `403 SELF_CLAIM`.
- Satu klaim terbuka (PENDING/APPROVED) per pengaju per barang → `409 DUPLICATE_CLAIM` (dijaga juga oleh partial unique index).
- Hanya **satu** klaim APPROVED/COMPLETED per barang → `409 ALREADY_APPROVED` (row lock `SELECT … FOR UPDATE` + partial unique index).
- Persetujuan ≠ barang sudah diambil. Laporan baru `COMPLETED` saat penemu memanggil `/complete`.
- `pickup.method` mengikuti lokasi aktual: `AT_PICKUP_POINT` jika `DEPOSITED`, selain itu `MEET_FINDER`. Bila penemu memindahkan barang setelah approve, pickup klaim diperbarui + notifikasi `PICKUP_CHANGED`.

**Fungsi 2 – Peta**
- Stok pos = laporan `FOUND` + `ACTIVE` + `custody=DEPOSITED`. `AWAITING_DROPOFF` belum dihitung.
- `GET /map/availability` hanya mengirim marker untuk pos dengan `available_count > 0`; jika kosong `is_empty=true` + `message`.
- Status selalu dibaca ulang ketika `GET /reports/{id}` dipanggil.

**Fungsi 3 – Inbox**
- Notifikasi dibuat di transaksi yang sama dengan perubahan status; `event_key` unik (tipe + klaim + penerima + status) sehingga tidak ada notifikasi ganda.
- `GET /claims/{id}` otomatis menandai notifikasi klaim tsb sebagai dibaca.
- `badge.total = unread_notifications + pending_incoming_claims`.

**Fungsi 4 – Rekomendasi**
- Filter wajib: jenis berlawanan, ACTIVE, bukan milik sendiri, kategori sama, `found.occurred_at ≥ lost.occurred_at`, selisih ≤ 30 hari.
- Pra-skor: kategori +2, warna sama +3, area sama +2 (atau nama lokasi mirip +1), selisih waktu ≤24j +2 / ≤72j +1, kemiripan kata kunci ×3.
- 10 kandidat teratas dikirim ke Gemini (hanya atribut publik; tanpa kontak & ciri rahasia). ID hasil divalidasi terhadap kandidat yang dikirim; label hanya HIGH/MEDIUM/LOW.
- Urutan: level → jumlah atribut cocok → pra-skor → selisih waktu terkecil → laporan terbaru.
- < 3 atribut inti bisa dibandingkan → maksimal `MEDIUM` + notice data belum lengkap.
- Tanpa `GEMINI_API_KEY` atau AI gagal → `source: "RULE_BASED"`.

**Akun & Lokasi**
- Registrasi wajib NIM (unik), fakultas (kode/nama dari `/faculties`), WhatsApp, `password_confirmation` sama, `agree_terms=true`. Email opsional.
- `/pickup-points` berisi 1 Pos Satpam per fakultas UB. Dipakai untuk dropdown Lokasi di form Lapor **dan** pos penitipan.
- `location_point_id` mengisi `location_area` = nama fakultas (dipakai pencocokan "area sama") dan koordinat bila kosong.

## 5. Kerahasiaan Data

| Tingkat | Data | Siapa bisa baca |
|---|---|---|
| Publik (login) | atribut laporan, foto laporan, nama & fakultas pemilik | semua user login |
| Terbatas | NIM, email, WhatsApp, Instagram | pemilik akun; kontak pelapor LOST via `/contact`; kontak penemu hanya untuk pengaju yang APPROVED |
| Rahasia | `secret_description`, `proof_photo_url` klaim | pengaju & penemu klaim tsb (pihak lain dapat 404) |

## 6. Alur Contoh (happy path)

1. Penemu `POST /uploads` → `POST /reports` (`type=FOUND`, `location_point_id`, `custody=AWAITING_DROPOFF`, `pickup_point_id`).
2. Penemu menitipkan barang → `POST /reports/{id}/custody/confirm-deposit` → muncul di `/map/availability`.
3. Pemilik `POST /reports` (`LOST`) → `POST /reports/{lostId}/matches` → buka kandidat `GET /reports/{foundId}`.
4. Pemilik `POST /reports/{foundId}/claims` (ciri rahasia) → penemu dapat notifikasi `CLAIM_SUBMITTED`.
5. Penemu `GET /claims/incoming` → `POST /claims/{id}/approve`.
6. Pemilik `GET /claims/{id}/proof` → ambil di pos.
7. Penemu `POST /claims/{id}/complete` → laporan `COMPLETED`, hilang dari peta.
