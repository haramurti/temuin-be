# Sumber foto data demo Temuin

`scripts/seed_chat_demo.sh` mengunduh foto ke direktori upload lokal backend. Skrip juga menyalin dua foto ilustratif ransel yang tersedia di repo. Feed, hasil kecocokan, dan halaman detail memakai URL foto dari laporan yang sama.

| File lokal | Isi | Sumber dan lisensi |
|---|---|---|
| `temuin_demo_wallet.jpg` | Dompet kulit cokelat | [Unsplash](https://images.unsplash.com/photo-1627123424574-724758594e93), Unsplash License |
| `temuin_demo_wallet_alt.jpg` | Dompet hitam pembanding | [Foto Oliur di Unsplash](https://unsplash.com/photos/black-leather-bifold-wallet-on-brown-wooden-table-9Huby3g9fN0), Unsplash License |
| `temuin_demo_bottle.jpg` | Botol minum hijau | [Unsplash](https://images.unsplash.com/photo-1602143407151-7111542de6e8), Unsplash License |
| `temuin_demo_calculator.jpg` | Kalkulator ilmiah hitam | [Foto Charles Deluvio di Unsplash](https://unsplash.com/photos/black-scientific-calculator-beside-black-headphones-zGn2cg9qBvU), Unsplash License |
| `temuin_demo_calculator_alt.jpg` | Kalkulator Casio putih pembanding | [Foto Annie Spratt di Unsplash](https://unsplash.com/photos/white-casio-calculator-r_m-JKJVto0), Unsplash License |
| `temuin_ransel_lost.jpg` | Ransel navy dari sudut depan-kiri untuk laporan kehilangan | Foto ilustratif dibuat khusus untuk kasus pencocokan lokal Temuin |
| `temuin_ransel_found.jpg` | Ransel yang sama dari sudut kanan-samping untuk laporan temuan | Foto ilustratif dibuat khusus untuk kasus pencocokan lokal Temuin |

Seed berisi 10 laporan. Dua laporan kategori Tas memakai deskripsi dengan kata-kata berbeda dan foto dari sudut berbeda untuk menguji kecocokan visual. Jalankan seed dengan `scripts/seed_chat_demo.sh`. Secara default foto tersimpan di `uploads/` agar cocok dengan konfigurasi backend lokal. Atur `TEMUIN_UPLOAD_DIR` bila `UPLOAD_DIR` backend memakai lokasi lain. Gunakan hanya pada database pengembangan lokal. Skrip dapat dijalankan ulang tanpa menambah duplikat.

## Akun demo lokal

Empat akun demo memakai NIM berakhiran `901` sampai `904` dengan kata sandi `password123`. Gunakan hanya pada database lokal yang sudah menjalankan seed demo.
