-- Seed idempotent untuk pengembangan lokal: 4 akun dan 10 laporan.
-- Jalankan melalui scripts/seed_chat_demo.sh agar foto lokal tersedia.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
BEGIN;

INSERT INTO users (id, name, email, password_hash, nim, faculty, whats_app, instagram, created_at, updated_at) VALUES
('33333333-0000-4000-8000-000000000001', 'Alya Rahma', 'demo.alya@student.ub.ac.id', crypt('password123', gen_salt('bf', 10)), '245150200111901', 'Fakultas Vokasi', '081200009001', NULL, now(), now()),
('33333333-0000-4000-8000-000000000002', 'Bima Prakoso', 'demo.bima@student.ub.ac.id', crypt('password123', gen_salt('bf', 10)), '245150200111902', 'Fakultas Teknik', '081200009002', NULL, now(), now()),
('33333333-0000-4000-8000-000000000003', 'Citra Nabila', 'demo.citra@student.ub.ac.id', crypt('password123', gen_salt('bf', 10)), '245150200111903', 'Fakultas Ilmu Budaya', '081200009003', NULL, now(), now()),
('33333333-0000-4000-8000-000000000004', 'Dafa Maulana', 'demo.dafa@student.ub.ac.id', crypt('password123', gen_salt('bf', 10)), '245150200111904', 'Fakultas Ilmu Komputer', '081200009004', NULL, now(), now())
ON CONFLICT DO NOTHING;

WITH pp AS (SELECT code, id, building, latitude, longitude FROM pickup_points),
sample_reports (id, uid, type, title, category, color, description, ago, loc_name, loc_code, custody, photo) AS (VALUES
('44444444-0000-4000-8000-000000000001', 1, 'LOST',  'Dompet kulit cokelat', 'WALLET', 'Cokelat', 'Dompet cokelat berisi KTM dan kartu perpustakaan, terakhir terlihat di meja kantin.', interval '2 hours', 'Kantin Vokasi, meja dekat jendela', 'SATPAM-VOKASI', NULL, '/uploads/reports/temuin_demo_wallet.jpg'),
('44444444-0000-4000-8000-000000000002', 2, 'LOST',  'Botol minum hijau', 'BOTTLE', 'Hijau', 'Botol minum hijau dengan stiker kecil berbentuk awan, tertinggal setelah kuliah.', interval '8 hours', 'Balairung, tangga sisi timur', 'SATPAM-FH', NULL, '/uploads/reports/temuin_demo_bottle.jpg'),
('44444444-0000-4000-8000-000000000003', 3, 'LOST',  'Kalkulator Casio hitam', 'ELECTRONICS', 'Hitam', 'Kalkulator ilmiah hitam dengan goresan di sudut kanan, kemungkinan tertinggal di lab.', interval '24 hours', 'Lab Fakultas Teknik, meja baris depan', 'SATPAM-FT', NULL, '/uploads/reports/temuin_demo_calculator.jpg'),
('44444444-0000-4000-8000-000000000004', 4, 'FOUND', 'Dompet cokelat ditemukan', 'WALLET', 'Cokelat', 'Dompet kulit warna cokelat ditemukan di area kantin. Pemilik dapat menjelaskan isi di dalamnya.', interval '90 minutes', 'Kantin Vokasi, meja dekat jendela', 'SATPAM-VOKASI', 'AWAITING_DROPOFF', '/uploads/reports/temuin_demo_wallet.jpg'),
('44444444-0000-4000-8000-000000000005', 1, 'FOUND', 'Botol hijau ditemukan', 'BOTTLE', 'Hijau', 'Botol minum hijau dengan tutup abu-abu ditemukan dekat tangga Balairung.', interval '2 hours', 'Balairung, tangga sisi timur', 'SATPAM-FH', 'AWAITING_DROPOFF', '/uploads/reports/temuin_demo_bottle.jpg'),
('44444444-0000-4000-8000-000000000006', 2, 'FOUND', 'Kalkulator hitam ditemukan', 'ELECTRONICS', 'Hitam', 'Kalkulator Casio hitam ditemukan di meja lab Fakultas Teknik.', interval '20 hours', 'Lab Fakultas Teknik, meja baris depan', 'SATPAM-FT', 'AWAITING_DROPOFF', '/uploads/reports/temuin_demo_calculator.jpg'),
('44444444-0000-4000-8000-000000000007', 3, 'FOUND', 'Dompet kartu warna hitam', 'WALLET', 'Hitam', 'Dompet kartu hitam tipis ditemukan di bangku taman, bukan di kantin.', interval '1 hour', 'Taman depan Perpustakaan', 'SATPAM-FMIPA', 'HELD_BY_FINDER', '/uploads/reports/temuin_demo_wallet_alt.jpg'),
('44444444-0000-4000-8000-000000000008', 4, 'FOUND', 'Kalkulator scientific putih', 'ELECTRONICS', 'Putih', 'Kalkulator scientific putih ditemukan di ruang belajar, merek dan cirinya berbeda.', interval '19 hours', 'Ruang belajar Fakultas Teknik', 'SATPAM-FT', 'HELD_BY_FINDER', '/uploads/reports/temuin_demo_calculator_alt.jpg'),
('44444444-0000-4000-8000-000000000009', 1, 'LOST', 'Ransel biru tua dengan patch jingga', 'BAG', 'Navy', 'Ransel berbahan kain biru tua dengan patch segitiga oranye di kantong depan. Terakhir terlihat di bangku dekat area baca.', interval '4 hours', 'Bangku depan Perpustakaan Vokasi', 'SATPAM-VOKASI', NULL, '/uploads/reports/temuin_ransel_lost.jpg'),
('44444444-0000-4000-8000-00000000000a', 2, 'FOUND', 'Tas punggung navy ditemukan', 'BAG', 'Biru tua', 'Tas punggung warna navy ditemukan dekat meja baca. Pada kantong depannya ada patch segitiga jingga.', interval '2 hours', 'Area baca Perpustakaan Vokasi', 'SATPAM-VOKASI', 'AWAITING_DROPOFF', '/uploads/reports/temuin_ransel_found.jpg')
)
INSERT INTO reports (id, user_id, type, title, category, color, description, occurred_at,
                     location_name, location_area, location_point_id, latitude, longitude, photo_url,
                     status, custody, pickup_point_id, deposited_at, created_at, updated_at)
SELECT d.id::uuid,
       ('33333333-0000-4000-8000-00000000000' || d.uid)::uuid,
       d.type, d.title, d.category, d.color, d.description, now() - d.ago,
       d.loc_name, pp.building, pp.id, pp.latitude, pp.longitude, :'base_url' || d.photo,
       'ACTIVE', d.custody,
       CASE WHEN d.custody = 'AWAITING_DROPOFF' THEN pp.id ELSE NULL END,
       NULL, now() - d.ago, now() - d.ago
FROM sample_reports d JOIN pp ON pp.code = d.loc_code
ON CONFLICT (id) DO NOTHING;

COMMIT;
