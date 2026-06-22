#!/usr/bin/env bash
# Re-encode existing capture clips in place so they play in strict browser
# decoders. Old clips were stream-copied (-c copy), which spliced raw H.264
# across segment joins and kept Opus audio — browsers stall on the glitch and
# can't play Opus-in-MP4 in Safari. Re-encoding to H.264 + AAC (the same flags
# the app now uses) produces a clean, fully-decodable file.
#
# Idempotent: a clip whose audio is already AAC came from the new encode path,
# so it's skipped. Safe to re-run. Re-encodes to a temp file and only replaces
# the original on success, so an interrupted run never corrupts a clip.
#
# Usage: reencode-clips.sh [MEDIA_DIR]
#   MEDIA_DIR defaults to the drivers media folder; pass it if yours differs.
#   e.g. reencode-clips.sh /config/media/drivers
set -euo pipefail

DIR="${1:-${XDG_CONFIG_HOME:-$HOME/.config}/sm/media/drivers}"
[ -d "$DIR" ] || { echo "media dir not found: $DIR (pass it as arg 1)" >&2; exit 1; }
command -v ffmpeg  >/dev/null || { echo "ffmpeg not on PATH"  >&2; exit 1; }
command -v ffprobe >/dev/null || { echo "ffprobe not on PATH" >&2; exit 1; }

fixed=0 skipped=0 failed=0
while IFS= read -r -d '' f; do
  acodec=$(ffprobe -v error -select_streams a:0 -show_entries stream=codec_name -of csv=p=0 "$f" || true)
  if [ "$acodec" = "aac" ]; then
    skipped=$((skipped+1)); continue
  fi
  tmp="$f.reencode.tmp.mp4"
  if ffmpeg -nostdin -y -loglevel error \
       -i "$f" \
       -c:v libx264 -preset veryfast -crf 23 -c:a aac -movflags +faststart \
       "$tmp" && [ -s "$tmp" ]; then
    mv -f "$tmp" "$f"
    echo "fixed: $f"
    fixed=$((fixed+1))
  else
    rm -f "$tmp"
    echo "FAILED: $f" >&2
    failed=$((failed+1))
  fi
done < <(find "$DIR" -type f \( -name '*_clip.mp4' -o -name '*_manual.mp4' \) -print0)

echo "done. fixed=$fixed skipped(already aac)=$skipped failed=$failed"
