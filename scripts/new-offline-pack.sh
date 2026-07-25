#!/bin/sh
set -eu
CDPATH=''

[ "$#" -eq 3 ] || {
  echo "Usage: new-offline-pack.sh INPUT_DIR metadata.json OUTPUT.tar.gz" >&2
  exit 2
}
input=$(cd -- "$1" && pwd -P)
metadata=$2
output=$3
command -v jq >/dev/null 2>&1 || { echo "jq is required to validate metadata" >&2; exit 9; }

stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT HUP INT TERM
mkdir -p "$stage/payload"
jq -e '.schemaVersion == 1 and (.components | length > 0)' "$metadata" >/dev/null
jq -c '.components[]' "$metadata" | while IFS= read -r component; do
  file=$(printf '%s' "$component" | jq -r '.file')
  source_url=$(printf '%s' "$component" | jq -r '.sourceUrl')
  license=$(printf '%s' "$component" | jq -r '.license')
  case "$file" in /*|../*|*/../*|*/..) echo "file must stay inside INPUT_DIR: $file" >&2; exit 2 ;; esac
  case "$source_url" in https://*) ;; *) echo "sourceUrl must use HTTPS: $file" >&2; exit 8 ;; esac
  if [ -z "$license" ] || [ "$license" = null ]; then
    echo "license missing: $file" >&2
    exit 2
  fi
  candidate_dir=$(cd -- "$(dirname "$input/$file")" 2>/dev/null && pwd -P) || { echo "payload directory missing: $file" >&2; exit 2; }
  candidate="$candidate_dir/$(basename "$file")"
  case "$candidate" in "$input"/*) ;; *) echo "file escapes INPUT_DIR: $file" >&2; exit 2 ;; esac
  [ -f "$candidate" ] && [ ! -L "$candidate" ] || { echo "payload missing or symbolic link rejected: $file" >&2; exit 2; }
  mkdir -p "$stage/payload/$(dirname "$file")"
  cp "$candidate" "$stage/payload/$file"
done

created=$(date -u +%Y-%m-%dT%H:%M:%SZ)
jq --arg created "$created" '
  {
    schemaVersion: 1,
    format: "ssh-launchpad-offline-pack",
    createdAt: $created,
    note: "Local user-created dependency payload. Verify license before redistributing.",
    components: [.components[] | . + {file: ("payload/" + .file)}]
  }' "$metadata" > "$stage/manifest.json"

: > "$stage/bundle-checksums.txt"
jq -r '.components[].file' "$metadata" | while IFS= read -r file; do
  if command -v sha256sum >/dev/null 2>&1; then
    hash=$(sha256sum "$stage/payload/$file" | awk '{print $1}')
  else
    hash=$(shasum -a 256 "$stage/payload/$file" | awk '{print $1}')
  fi
  printf '%s  payload/%s\n' "$hash" "$file" >> "$stage/bundle-checksums.txt"
done
script_dir=$(cd -- "$(dirname "$0")" && pwd -P)
find_help() {
  for candidate in "$script_dir/../docs/$1" "$script_dir/$1" "$script_dir/$2"; do
    if [ -f "$candidate" ]; then printf '%s\n' "$candidate"; return 0; fi
  done
  return 1
}
help_zh=$(find_help offline-help.zh-CN.md "离线帮助-中文.md") || { echo "offline-help.zh-CN.md is missing" >&2; exit 2; }
help_en=$(find_help offline-help.en.md "Offline Help - English.md") || { echo "offline-help.en.md is missing" >&2; exit 2; }
cp "$help_zh" "$stage/offline-help.zh-CN.md"
cp "$help_en" "$stage/offline-help.en.md"
tar -C "$stage" -czf "$output" .
echo "Offline pack created: $output"
