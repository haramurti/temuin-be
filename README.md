# Temuin API

Backend **Temuin – Smart Geo-Tagged Lost & Found System** (UB). Go + Fiber v2 + GORM + PostgreSQL, arsitektur DDD.

- API contract: [docs/API_CONTRACT.md](docs/API_CONTRACT.md)
- OpenAPI 3.0 (import ke Apidog: *Import → OpenAPI/Swagger → file*): [docs/openapi.yaml](docs/openapi.yaml)

## Jalankan pakai Docker (API + Postgres)

```bash
cp .env.example .env
make up         # = docker compose up -d --build → API http://localhost:8080
make logs       # log API
make down
```
Di dalam compose `DB_HOST/DB_PORT` otomatis di-override ke `postgres:5432`; foto tersimpan di volume `temuin_uploads`.

## Akses dari device/laptop lain (ngrok)

```bash
make up
make tunnel     # = ngrok http 8080 → https://<nama>.ngrok-free.dev
```
1. Set `BASE_URL=https://<nama>.ngrok-free.dev` di `.env` lalu `docker compose up -d api` (supaya URL foto upload ikut publik).
2. Base URL untuk app: `https://<nama>.ngrok-free.dev/api/v1`.
3. ngrok free menampilkan halaman peringatan untuk browser; client app sebaiknya kirim header `ngrok-skip-browser-warning: 1`.
4. Laptop ini harus tetap nyala + Docker & ngrok jalan.

## Jalankan lokal tanpa Docker untuk API

```bash
cp .env.example .env
make db-up      # Postgres 16 di localhost:5433 (docker compose, project "temuin")
make run        # API di http://localhost:8080
```
Tabel dibuat otomatis (`AUTO_MIGRATE=true`) dan 17 Pos Satpam (satu per fakultas UB) di-seed sebagai pilihan lokasi & pos penitipan (`SEED_DATA=true`).
Rekomendasi kecocokan dan Chat Bantuan Temuin sama-sama memakai DeepSeek melalui SinarCloud, dengan `SINARCLOUD_API_KEY` dan model yang dipilih lewat `DEEPSEEK_MODEL` (default `deepseek/deepseek-v4-1-flash`). Rekomendasi mengirim maksimal sepuluh kandidat beratribut publik ke model, lalu memvalidasi ID dan tingkat kecocokan sebelum menampilkan hasil. Untuk pertanyaan tentang barang, backend menyertakan konteks dari maksimal empat laporan publik aktif dan mengembalikan referensi yang dapat dibuka di aplikasi. Data kontak serta ciri rahasia klaim tidak dikirim ke model. Tanpa key atau saat provider gagal, rekomendasi memakai aturan dan chat menjawab dari laporan aktif atau panduan FAQ sesuai pertanyaan.

Health check: `GET /health`.

## Struktur (DDD)

```
cmd/api/main.go                    composition root (wiring dependency)
internal/
  domain/                          ENTITAS + aturan bisnis + interface repository (tanpa Fiber/GORM)
    user/ report/ claim/ pickup/ notification/ matching/
  application/                     use case / service per bounded context
    auth/ user/ report/ claim/ inbox/ pickup/ matching/ events/ dto/
  infrastructure/                  implementasi port
    persistence/postgres/          GORM repo, migrasi, partial unique index, seed, TxManager
    security/                      bcrypt + JWT
    storage/                       upload gambar ke disk lokal
    ai/                            SinarCloud DeepSeek client for matching and help
  interfaces/http/                 Fiber: router, handler, middleware, response envelope
  shared/                          apperror, tx
docs/                              openapi.yaml, API_CONTRACT.md
```

Alur dependensi: `interfaces → application → domain ← infrastructure`.
Domain tidak meng-import GORM/Fiber (tag `gorm:"..."` di struct hanya metadata string).

## Catatan deploy
- Ganti `JWT_SECRET`, set `APP_ENV=production`, `BASE_URL` ke domain publik, `CORS_ORIGINS` spesifik.
- `uploads/` disimpan di disk lokal — pakai volume persisten atau ganti `storage` ke S3/GCS.
- Koordinat pos satpam fakultas di `seed.go` masih perkiraan; sesuaikan dengan pengelola pos.
