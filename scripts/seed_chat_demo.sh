#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
db_container="${TEMUIN_DB_CONTAINER:-temuin-postgres}"
upload_dir="${TEMUIN_UPLOAD_DIR:-$repo_dir/uploads}"
base_url="${TEMUIN_BASE_URL:-http://10.0.2.2:8080}"
image_dir="$upload_dir/reports"

mkdir -p "$image_dir"
curl -fLsS --retry 2 --max-time 30 'https://images.unsplash.com/photo-1627123424574-724758594e93?w=600&auto=format&fit=crop&q=85' -o "$image_dir/temuin_demo_wallet.jpg"
curl -fLsS --retry 2 --max-time 30 'https://images.unsplash.com/photo-1612023395494-1c4050b68647?w=600&auto=format&fit=crop&q=85' -o "$image_dir/temuin_demo_wallet_alt.jpg"
curl -fLsS --retry 2 --max-time 30 'https://images.unsplash.com/photo-1602143407151-7111542de6e8?w=600&auto=format&fit=crop&q=85' -o "$image_dir/temuin_demo_bottle.jpg"
curl -fLsS --retry 2 --max-time 30 'https://images.unsplash.com/photo-1511377107391-116a9d5d20b5?w=600&auto=format&fit=crop&q=85' -o "$image_dir/temuin_demo_calculator.jpg"
curl -fLsS --retry 2 --max-time 30 'https://images.unsplash.com/photo-1574607383077-47ddc2dc51c4?w=600&auto=format&fit=crop&q=85' -o "$image_dir/temuin_demo_calculator_alt.jpg"
cp "$repo_dir/scripts/assets/temuin_ransel_lost.jpg" "$image_dir/"
cp "$repo_dir/scripts/assets/temuin_ransel_found.jpg" "$image_dir/"

docker exec -i "$db_container" psql -v ON_ERROR_STOP=1 -v "base_url=${base_url%/}" -U temuin -d temuin \
    < "$repo_dir/docs/seed_chat_demo.sql"
