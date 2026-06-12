#!/bin/sh
# Install mdserve from GitHub Releases.
#
#   curl -fsSL https://raw.githubusercontent.com/mostafa-re/mdserve/main/scripts/install.sh | sh
#
# Options (environment):
#   MDSERVE_VERSION   tag to install (default: latest release)
#   MDSERVE_BINDIR    install dir     (default: ~/.local/bin)
set -eu

REPO="mostafa-re/mdserve"
BINDIR="${MDSERVE_BINDIR:-$HOME/.local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$os" in
  linux)  os=linux ;;
  darwin) os=darwin ;;
  *) echo "mdserve: unsupported OS '$os' — see https://github.com/$REPO/releases" >&2; exit 1 ;;
esac
case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "mdserve: unsupported architecture '$arch'" >&2; exit 1 ;;
esac

ver="${MDSERVE_VERSION:-}"
if [ -z "$ver" ]; then
  ver="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
        | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
fi
[ -n "$ver" ] || { echo "mdserve: could not determine the latest version" >&2; exit 1; }

asset="mdserve_${ver}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$ver/$asset"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "mdserve: downloading $ver ($os/$arch)"
curl -fSL "$url" -o "$tmp/$asset"
tar -xzf "$tmp/$asset" -C "$tmp"

mkdir -p "$BINDIR"
mv "$tmp/mdserve" "$BINDIR/mdserve"
chmod +x "$BINDIR/mdserve"

echo "mdserve: installed $ver to $BINDIR/mdserve"
case ":$PATH:" in
  *":$BINDIR:"*) ;;
  *) echo "mdserve: add $BINDIR to your PATH, e.g.  export PATH=\"$BINDIR:\$PATH\"" ;;
esac
