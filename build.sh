#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"

echo "1/3 Building client..."
cd "$SCRIPT_DIR/client"
npm install
npm run build

echo "2/3 Building native capture..."
cd "$SCRIPT_DIR"
dotnet build "native-capture/tests/CaptureProbe/CaptureProbe.csproj"

echo "3/3 Building host..."
cd "$SCRIPT_DIR/host"
go build ./...

echo "Done."
