#!/bin/sh
set -eu

[ "$#" -eq 3 ] || {
  echo "Usage: new-offline-pack.sh INPUT_DIR metadata.json OUTPUT.tar.gz" >&2
  exit 2
}
input=$1
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
  [ -n "$license" ] && [ "$license" != null ] || { echo "license missing: $file" >&2; exit 2; }
  [ -f "$input/$file" ] || { echo "payload missing: $file" >&2; exit 2; }
  mkdir -p "$stage/payload/$(dirname "$file")"
  cp "$input/$file" "$stage/payload/$file"
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
    hash=$(sha256sum "$input/$file" | awk '{print $1}')
  else
    hash=$(shasum -a 256 "$input/$file" | awk '{print $1}')
  fi
  printf '%s  payload/%s\n' "$hash" "$file" >> "$stage/bundle-checksums.txt"
done
cp docs/offline-help.zh-CN.md docs/offline-help.en.md "$stage/"
tar -C "$stage" -czf "$output" .
echo "Offline pack created: $output"
