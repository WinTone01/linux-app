#!/usr/bin/env bash
# Installs this geteduroam client on Arch Linux and distributions based on it
# (CachyOS, EndeavourOS, Manjaro, ...) as a real pacman package.
#
#   ./install-arch.sh              build and install
#   ./install-arch.sh --uninstall  remove the package again
#
# Connection profiles and certificates in ~/.local/share/geteduroam are never
# touched: NetworkManager still needs them.
set -euo pipefail

REPO="https://github.com/WinTone01/linux-app.git"
PKG="geteduroam-git"

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '==> %s\n' "$*"; }

command -v pacman >/dev/null 2>&1 || die "pacman not found, this script is only for Arch based distributions"
[ "$(id -u)" -ne 0 ] || die "do not run this as root, you are asked for your password when it is needed"
command -v sudo >/dev/null 2>&1 || die "sudo is required"

case "${1:-}" in
"") ;;
--uninstall)
	pacman -Qq "$PKG" >/dev/null 2>&1 || die "$PKG is not installed"
	exec sudo pacman -Rns "$PKG"
	;;
-h | --help)
	printf 'Usage: %s [--uninstall]\n' "$0"
	exit 0
	;;
*) die "unknown argument: $1" ;;
esac

# Binaries that were put in ~/.local by hand go before the package in $PATH,
# and their desktop file hides the one from the package
remove_manual_install() {
	local data="${XDG_DATA_HOME:-$HOME/.local/share}" f
	local files=()
	for f in "$HOME"/.local/bin/geteduroam-{cli,gui,notifcheck} \
		"$data/applications/app.eduroam.geteduroam.desktop" \
		"$data"/icons/hicolor/*/apps/app.eduroam.geteduroam.png; do
		[ -e "$f" ] && files+=("$f")
	done
	[ "${#files[@]}" -gt 0 ] || return 0
	info "Removing the old manual installation:"
	printf '    %s\n' "${files[@]}"
	rm -f -- "${files[@]}"
}

info "Installing the build dependencies"
sudo pacman -S --needed base-devel git go networkmanager gtk4 libadwaita libnotify

# From a checkout the local source is built, otherwise (curl | bash) it is cloned
root="$(cd "$(dirname "${BASH_SOURCE[0]:-/nonexistent/x}")" 2>/dev/null && pwd || true)"
if [ -z "$root" ] || [ ! -f "$root/packaging/arch/PKGBUILD" ]; then
	tmp="$(mktemp -d)"
	# shellcheck disable=SC2064
	trap "rm -rf -- '$tmp'" EXIT
	info "Cloning $REPO"
	git clone "$REPO" "$tmp/linux-app"
	root="$tmp/linux-app"
fi

info "Building the package"
cd "$root/packaging/arch"
# --force rebuilds when the version did not change, e.g. with uncommitted changes
makepkg --syncdeps --install --clean --force

remove_manual_install
hash -r
info "Done, start geteduroam from your application menu or run geteduroam-gui"
