#!/bin/sh
# Restart the main.js script using mpd
# Use the -f flag to follow the journal logs
set -e

FLAG=""
if [ "$1" = "-f" ]; then
 FLAG="-f"
fi

sudo systemctl restart mpc-ir
journalctl -u mpc-ir $FLAG