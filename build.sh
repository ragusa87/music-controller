#!/bin/bash
set -euo pipefail

mkdir -p bin
go build -o bin/controller main.go

mkdir -p "$HOME/.config/systemd/user/"

cp ./*.service "$HOME/.config/systemd/user/"

systemctl --user daemon-reload

for i in ./*.service; do
  systemctl --user enable --now "$(basename "$i")"
done

