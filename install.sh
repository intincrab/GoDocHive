#!/bin/sh
# install.sh - download the latest GoDocHive (hiver) binary for this OS/arch.
#
#   curl -fsSL https://raw.githubusercontent.com/intincrab/GoDocHive/main/install.sh | sh
#
# environment overrides:
#   VERSION  tag to install (default: latest release, e.g. v0.2.0)
#   BINDIR   install directory (default: /usr/local/bin if writable, else ~/.local/bin)

set -eu

REPO="intincrab/GoDocHive"
PROJECT="GoDocHive"
BINARY="hiver"

err() { echo "install: $*" >&2; exit 1; }

# --- detect os / arch ---
os=$(uname -s)
case "$os" in
	Linux) os="linux" ;;
	Darwin) os="darwin" ;;
	*) err "unsupported OS '$os' - grab a binary from https://github.com/$REPO/releases" ;;
esac

arch=$(uname -m)
case "$arch" in
	x86_64 | amd64) arch="amd64" ;;
	aarch64 | arm64) arch="arm64" ;;
	*) err "unsupported architecture '$arch'" ;;
esac

# --- pick a downloader ---
if command -v curl >/dev/null 2>&1; then
	dl() { curl -fsSL "$1"; }
	dlo() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	dl() { wget -qO- "$1"; }
	dlo() { wget -qO "$2" "$1"; }
else
	err "need curl or wget installed"
fi

# --- resolve version ---
version="${VERSION:-}"
if [ -z "$version" ]; then
	version=$(dl "https://api.github.com/repos/$REPO/releases/latest" |
		grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/')
	[ -n "$version" ] || err "could not determine the latest version"
fi
ver_no_v="${version#v}"

# --- download & extract ---
asset="${PROJECT}_${ver_no_v}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$version/$asset"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "downloading $asset ..."
dlo "$url" "$tmp/$asset" || err "download failed: $url"
tar -xzf "$tmp/$asset" -C "$tmp" || err "failed to extract $asset"
[ -f "$tmp/$BINARY" ] || err "binary '$BINARY' not found in archive"

# --- choose install dir ---
bindir="${BINDIR:-}"
if [ -z "$bindir" ]; then
	if [ -w /usr/local/bin ]; then
		bindir="/usr/local/bin"
	else
		bindir="$HOME/.local/bin"
	fi
fi
mkdir -p "$bindir"
cp "$tmp/$BINARY" "$bindir/$BINARY"
chmod 0755 "$bindir/$BINARY"

echo "installed $BINARY $version to $bindir/$BINARY"
case ":$PATH:" in
	*":$bindir:"*) ;;
	*) echo "note: $bindir is not on your PATH - add it, e.g.  export PATH=\"$bindir:\$PATH\"" ;;
esac
echo "run  $BINARY -path /path/to/your/docs  then open http://127.0.0.1:3030/search"
