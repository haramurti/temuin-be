-- Temuin demo data: 4 akun + 12 laporan (hilang & ditemukan) mengikuti feed Figma.
-- Aman dijalankan berulang (ID tetap + ON CONFLICT DO NOTHING).
-- Jalankan:  docker exec -i temuin-postgres psql -U temuin -d temuin < docs/seed_demo.sql
-- Semua akun demo: kata sandi = password123  (login pakai NIM)

CREATE EXTENSION IF NOT EXISTS pgcrypto;

BEGIN;

-- ---------- Users ----------
INSERT INTO users (id, name, email, password_hash, nim, faculty, whats_app, instagram, created_at, updated_at) VALUES
('11111111-0000-4000-8000-000000000001', 'Dimas Pratama',  'dimas@student.ub.ac.id',  crypt('password123', gen_salt('bf', 10)), '245150200111001', 'Fakultas Ilmu Komputer', '081211110001', '@dimas.p',  now(), now()),
('11111111-0000-4000-8000-000000000002', 'Salsa Ramadhani','salsa@student.ub.ac.id',  crypt('password123', gen_salt('bf', 10)), '245150200111002', 'Fakultas Teknik',        '081211110002', '@salsaa',   now(), now()),
('11111111-0000-4000-8000-000000000003', 'Rafi Akbar',     'rafi@student.ub.ac.id',   crypt('password123', gen_salt('bf', 10)), '245150200111003', 'Fakultas Ilmu Budaya',   '081211110003', NULL,        now(), now()),
('11111111-0000-4000-8000-000000000004', 'Nadia Putri',    'nadia@student.ub.ac.id',  crypt('password123', gen_salt('bf', 10)), '245150200111004', 'Fakultas Vokasi',        '081211110004', '@nadiaptr', now(), now())
ON CONFLICT DO NOTHING;

-- ---------- Reports ----------
-- Lokasi pakai pos satpam fakultas (location_point_id) + koordinatnya.
WITH pp AS (SELECT code, id, building, latitude, longitude FROM pickup_points),
data (id, uid, type, title, category, color, description, ago, loc_name, loc_code, custody, pickup_code, photo) AS (VALUES
  -- DICARI (LOST)
  ('22222222-0000-4000-8000-000000000001', 1, 'LOST',  'Dompet Kulit Eiger Coklat',   'WALLET',      'Coklat',      'Dompet kulit Eiger warna coklat, isi KTM dan beberapa kartu.',        interval '2 hours',  'Kantin Vokasi, Meja 12',      'SATPAM-VOKASI', NULL,               NULL,            'https://images.unsplash.com/photo-1627123424574-724758594e93?w=600'),
  ('22222222-0000-4000-8000-000000000002', 3, 'LOST',  'Kacamata Frame Hitam Doff',   'ACCESSORIES', 'Hitam',       'Kacamata minus frame hitam doff, ada case abu-abu.',                  interval '1 day',    'Perpus Pusat Lt 3, Meja 14',  'SATPAM-FMIPA',  NULL,               NULL,            'https://images.unsplash.com/photo-1574258495973-f010dfbb5371?w=600'),
  ('22222222-0000-4000-8000-000000000003', 2, 'LOST',  'Jaket Kuning Makara (Size L)','CLOTHING',    'Kuning',      'Jaket almamater kuning ukuran L, ada pin di saku kiri.',              interval '2 days',   'Halte Bus Kuning Teknik',     'SATPAM-FT',     NULL,               NULL,            'https://images.unsplash.com/photo-1591047139829-d91aecb6caea?w=600'),
  ('22222222-0000-4000-8000-000000000004', 4, 'LOST',  'Kunci Motor Honda + Gantungan','KEYS',       'Hitam',       'Kunci motor Honda dengan gantungan boneka beruang.',                  interval '5 hours',  'Parkiran FISIP',              'SATPAM-FISIP',  NULL,               NULL,            NULL),
  ('22222222-0000-4000-8000-000000000005', 1, 'LOST',  'KTM a.n. Dimas Pratama',      'ID_CARD',     'Biru',        'Kartu tanda mahasiswa UB, kemungkinan jatuh di gedung F.',             interval '3 days',   'Gedung F FILKOM, Lobby',      'SATPAM-FILKOM', NULL,               NULL,            NULL),
  -- DITEMUKAN (FOUND)
  ('22222222-0000-4000-8000-000000000006', 2, 'FOUND', 'Botol LocknLock Hijau Mint',  'BOTTLE',      'Hijau Mint',  'Botol minum LocknLock warna hijau mint, ada stiker di tutup.',        interval '3 hours',  'Balairung Utama, Tangga',     'SATPAM-FH',     'DEPOSITED',        'SATPAM-FH',     'https://images.unsplash.com/photo-1602143407151-7111542de6e8?w=600'),
  ('22222222-0000-4000-8000-000000000007', 3, 'FOUND', 'Casio fx-991EX ClassWiz',     'ELECTRONICS', 'Hitam',       'Kalkulator Casio fx-991EX, tertinggal di meja lab.',                   interval '1 day',    'Lab Mesin FT, Rak 4',         'SATPAM-FT',     'DEPOSITED',        'SATPAM-FT',     'https://images.unsplash.com/photo-1564939558297-fc396f18e5c7?w=600'),
  ('22222222-0000-4000-8000-000000000008', 4, 'FOUND', 'Flashdisk Sandisk Cruzer 32GB','ELECTRONICS','Merah',       'Flashdisk Sandisk merah-hitam, ditemukan di colokan PC.',             interval '3 days',   'Gedung A FIB, R. 203',        'SATPAM-FIB',    'DEPOSITED',        'SATPAM-FIB',    'https://images.unsplash.com/photo-1618410320928-25228d811631?w=600'),
  ('22222222-0000-4000-8000-000000000009', 1, 'FOUND', 'Dompet Coklat Merk Eiger',    'WALLET',      'Coklat',      'Dompet kulit coklat, ditemukan di bangku kantin.',                    interval '90 minutes','Kantin Vokasi',              'SATPAM-VOKASI', 'AWAITING_DROPOFF', 'SATPAM-VOKASI', NULL),
  ('22222222-0000-4000-8000-000000000010', 2, 'FOUND', 'Payung Lipat Navy',           'OTHER',       'Navy',        'Payung lipat warna navy, tertinggal di kelas.',                        interval '6 hours',  'Gedung FEB, Ruang E2.4',      'SATPAM-FEB',    'HELD_BY_FINDER',   NULL,            NULL),
  ('22222222-0000-4000-8000-000000000011', 3, 'FOUND', 'Tote Bag Kanvas Krem',        'BAG',         'Krem',        'Tote bag kanvas berisi buku catatan, ditemukan di musala.',           interval '2 days',   'Musala FIA',                  'SATPAM-FIA',    'DEPOSITED',        'SATPAM-FIA',    NULL),
  ('22222222-0000-4000-8000-000000000012', 4, 'FOUND', 'Buku Kalkulus Purcell Jilid 1','BOOKS',      'Biru',        'Buku Kalkulus Purcell jilid 1, ada nama di halaman depan.',           interval '4 days',   'Gazebo FMIPA',                'SATPAM-FMIPA',  'DEPOSITED',        'SATPAM-FMIPA',  NULL)
)
INSERT INTO reports (id, user_id, type, title, category, color, description, occurred_at,
                     location_name, location_area, location_point_id, latitude, longitude, photo_url,
                     status, custody, pickup_point_id, deposited_at, created_at, updated_at)
SELECT d.id::uuid,
       ('11111111-0000-4000-8000-00000000000' || d.uid)::uuid,
       d.type, d.title, d.category, d.color, d.description, now() - d.ago,
       d.loc_name, l.building, l.id, l.latitude, l.longitude, d.photo,
       'ACTIVE', d.custody, p.id,
       CASE WHEN d.custody = 'DEPOSITED' THEN now() - d.ago + interval '30 minutes' END,
       now() - d.ago, now() - d.ago
FROM data d
JOIN pp l ON l.code = d.loc_code
LEFT JOIN pp p ON p.code = d.pickup_code
ON CONFLICT DO NOTHING;

COMMIT;

-- Cek:
-- SELECT type, custody, count(*) FROM reports WHERE id::text LIKE '22222222-%' GROUP BY 1,2;
