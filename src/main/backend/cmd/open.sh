#!/usr/bin/env bash
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/../../../.." && pwd)"
DATA_DIR="$PROJECT_DIR/wisdb"
mkdir -p "$DATA_DIR"
cd "$SCRIPT_DIR"
go run launcher.go -open "$DATA_DIR/nya"
