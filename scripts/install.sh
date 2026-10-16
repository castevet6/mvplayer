#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_DIR=$(dirname -- "$SCRIPT_DIR")
INSTALL_PATH=/usr/local/bin/mvplayer

make -C "$PROJECT_DIR" build

if [ "$(id -u)" -eq 0 ]; then
	install -m 0755 "$PROJECT_DIR/build/mvplayer" "$INSTALL_PATH"
elif command -v sudo >/dev/null 2>&1; then
	sudo install -m 0755 "$PROJECT_DIR/build/mvplayer" "$INSTALL_PATH"
else
	printf '%s\n' 'Installing to /usr/local/bin requires root privileges (sudo was not found).' >&2
	exit 1
fi

printf 'Installed %s\n' "$INSTALL_PATH"
