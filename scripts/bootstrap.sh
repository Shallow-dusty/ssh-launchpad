#!/bin/sh
set -eu

VERSION="${SSH_LAUNCHPAD_VERSION:-0.1.0}"
INSTALL_DIR="${SSH_LAUNCHPAD_INSTALL_DIR:-$HOME/.local/bin}"
STRATEGY="${SSH_LAUNCHPAD_DOWNLOAD_STRATEGY:-official}"
BASE_URL="${SSH_LAUNCHPAD_BASE_URL:-https://github.com/Shallow-dusty/ssh-launchpad/releases/download}"
PROXY_URL="${SSH_LAUNCHPAD_PROXY_URL:-}"
OFFLINE_BUNDLE="${SSH_LAUNCHPAD_OFFLINE_BUNDLE:-}"
CACHE_DIR="${SSH_LAUNCHPAD_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/ssh-launchpad}"
RUN_STAGE="${SSH_LAUNCHPAD_RUN:-check}"
PROFILE="${SSH_LAUNCHPAD_PROFILE:-}"

usage() {
  echo "Usage: bootstrap.sh [--version V] [--install-dir DIR] [--strategy official|mirror|proxy|offline|cache]"
  echo "                    [--base-url HTTPS_URL] [--proxy URL] [--offline-bundle FILE]"
  echo "                    [--run check|plan|verify|none] [--profile FILE]"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --strategy) STRATEGY="$2"; shift 2 ;;
    --base-url) BASE_URL="$2"; shift 2 ;;
    --proxy) PROXY_URL="$2"; shift 2 ;;
    --offline-bundle) OFFLINE_BUNDLE="$2"; shift 2 ;;
    --cache-dir) CACHE_DIR="$2"; shift 2 ;;
    --run) RUN_STAGE="$2"; shift 2 ;;
    --profile) PROFILE="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) echo "Unsupported operating system: $(uname -s)" >&2; exit 9 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 9 ;;
esac

ASSET="ssh-launchpad_${VERSION}_${OS}_${ARCH}.tar.gz"
TAG="v${VERSION}"
ARCHIVE="$CACHE_DIR/$ASSET"
MANIFEST="$CACHE_DIR/checksums.txt"
mkdir -p "$CACHE_DIR" "$INSTALL_DIR"

download() {
  uri="$1"
  destination="$2"
  case "$uri" in
    https://*) ;;
    *) echo "Refusing non-HTTPS download: $uri" >&2; exit 8 ;;
  esac
  proxy_args=""
  if [ -n "$PROXY_URL" ]; then
    proxy_args="--proxy $PROXY_URL"
  fi
  # shellcheck disable=SC2086
  curl --fail --location --retry 3 --retry-all-errors --continue-at - $proxy_args --output "$destination" "$uri"
}

case "$STRATEGY" in
  offline)
    [ -f "$OFFLINE_BUNDLE" ] || { echo "offline strategy requires --offline-bundle" >&2; exit 2; }
    ARCHIVE="$OFFLINE_BUNDLE"
    MANIFEST="$(dirname "$ARCHIVE")/checksums.txt"
    [ -f "$MANIFEST" ] || { echo "checksums.txt must be next to the offline asset" >&2; exit 8; }
    ;;
  cache)
    [ -f "$ARCHIVE" ] && [ -f "$MANIFEST" ] || { echo "verified cache is incomplete" >&2; exit 8; }
    ;;
  official|mirror|proxy)
    case "$BASE_URL" in
      https://*) ;;
      *) echo "base URL must use HTTPS" >&2; exit 8 ;;
    esac
    [ "$STRATEGY" != "proxy" ] || [ -n "$PROXY_URL" ] || { echo "proxy strategy requires --proxy" >&2; exit 2; }
    release_base="${BASE_URL%/}/$TAG"
    download "$release_base/checksums.txt" "$MANIFEST"
    download "$release_base/$ASSET" "$ARCHIVE"
    ;;
  *) echo "Unsupported strategy: $STRATEGY" >&2; exit 2 ;;
esac

expected="$(awk -v asset="$ASSET" '{ name=$2; sub(/^\*/, "", name); sub(/\r$/, "", name); if (name == asset) { print $1; exit } }' "$MANIFEST")"
[ -n "$expected" ] || { echo "No SHA-256 entry for $ASSET" >&2; exit 8; }
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')"
fi
[ "$actual" = "$expected" ] || { echo "SHA-256 mismatch for $ASSET" >&2; exit 8; }
echo "Verified $ASSET ($actual)"

stage="$CACHE_DIR/extract-$VERSION-$ARCH"
rm -rf "$stage"
mkdir -p "$stage"
tar -xzf "$ARCHIVE" -C "$stage"
binary="$(find "$stage" -type f -name ssh-launchpad | head -n 1)"
[ -n "$binary" ] || { echo "release archive did not contain ssh-launchpad" >&2; exit 8; }
install -m 0755 "$binary" "$INSTALL_DIR/ssh-launchpad"

if [ "$RUN_STAGE" != "none" ]; then
  if [ -n "$PROFILE" ]; then
    "$INSTALL_DIR/ssh-launchpad" "$RUN_STAGE" --profile "$PROFILE" --output -
  else
    "$INSTALL_DIR/ssh-launchpad" "$RUN_STAGE" --output -
  fi
fi

echo "Installed: $INSTALL_DIR/ssh-launchpad"
