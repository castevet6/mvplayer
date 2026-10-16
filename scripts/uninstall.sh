#!/bin/sh
set -eu

INSTALL_PATH=/usr/local/bin/mvplayer

if [ ! -e "$INSTALL_PATH" ]; then
	printf '%s\n' "MVPLAYER is not installed at $INSTALL_PATH"
	exit 0
fi

if [ "$(id -u)" -eq 0 ]; then
	rm -- "$INSTALL_PATH"
elif command -v sudo >/dev/null 2>&1; then
	sudo rm -- "$INSTALL_PATH"
else
	printf '%s\n' 'Removing from /usr/local/bin requires root privileges (sudo was not found).' >&2
	exit 1
fi

printf 'Removed %s\n' "$INSTALL_PATH"
