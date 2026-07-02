#!/usr/bin/env bash
# Backup harian database cloud_pos → Google Drive (via rclone).
# Dijadwalkan cron tiap 00:00 WIB. Aman dijalankan manual kapan pun.
#
# Alur: pg_dump (format custom, sudah terkompresi) → simpan lokal →
# upload ke remote rclone "gdrive:" → pangkas backup lama (lokal 7 hari,
# Drive 30 hari). Bila remote gdrive belum dikonfigurasi, backup lokal
# tetap dibuat dan script mencatat peringatan (tidak gagal).
set -u

DB_CONTAINER="cloud-pos-db-1"
DB_NAME="cloud_pos"
DB_USER="postgres"
LOCAL_DIR="/home/candra/backups/db"
REMOTE="gdrive:cloud-pos-backups"
KEEP_LOCAL_DAYS=7
KEEP_REMOTE_DAYS=30
LOG="/home/candra/backups/backup.log"

STAMP=$(date +%F_%H%M)
FILE="$LOCAL_DIR/cloud_pos_${STAMP}.dump"

mkdir -p "$LOCAL_DIR"
log() { echo "[$(date '+%F %T')] $*" >> "$LOG"; }

# 1) Dump (format custom -Fc: terkompresi + bisa pg_restore selektif)
if ! docker exec "$DB_CONTAINER" pg_dump -U "$DB_USER" -Fc "$DB_NAME" > "$FILE" 2>>"$LOG"; then
  log "GAGAL: pg_dump error"
  rm -f "$FILE"
  exit 1
fi

# Sanity check: dump kosong/terpotong jangan dianggap sukses
SIZE=$(stat -c %s "$FILE" 2>/dev/null || echo 0)
if [ "$SIZE" -lt 10240 ]; then
  log "GAGAL: dump terlalu kecil (${SIZE} bytes) — dibatalkan"
  rm -f "$FILE"
  exit 1
fi
log "OK: dump $FILE ($(numfmt --to=iec "$SIZE" 2>/dev/null || echo "$SIZE B"))"

# 2) Upload ke Google Drive
if rclone listremotes 2>/dev/null | grep -q '^gdrive:'; then
  if rclone copy "$FILE" "$REMOTE" --timeout 10m >> "$LOG" 2>&1; then
    log "OK: terupload ke $REMOTE"
    # 3) Pangkas backup lama di Drive
    rclone delete "$REMOTE" --min-age "${KEEP_REMOTE_DAYS}d" >> "$LOG" 2>&1 || true
  else
    log "PERINGATAN: upload ke Google Drive gagal — backup lokal tetap ada"
  fi
else
  log "PERINGATAN: remote 'gdrive' belum dikonfigurasi (jalankan: rclone config) — backup hanya lokal"
fi

# 4) Pangkas backup lokal
find "$LOCAL_DIR" -name 'cloud_pos_*.dump' -mtime +"$KEEP_LOCAL_DAYS" -delete
log "Selesai."
