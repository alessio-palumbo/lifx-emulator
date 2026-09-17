#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: $0 <app-path> <archive-path>" >&2
  exit 2
fi
APP_PATH="$1"
ARCHIVE_PATH="$2"
: "${APPLE_ID:?APPLE_ID is required}"
: "${APPLE_TEAM_ID:?APPLE_TEAM_ID is required}"
: "${APPLE_APP_SPECIFIC_PASSWORD:?APPLE_APP_SPECIFIC_PASSWORD is required}"
[[ -d "$APP_PATH" ]] || { echo "App bundle not found" >&2; exit 1; }

SIGNING_IDENTITY="${MACOS_SIGNING_IDENTITY:-}"
if [[ -z "$SIGNING_IDENTITY" ]]; then
  SIGNING_IDENTITY="$({ security find-identity -v -p codesigning || true; } \
    | sed -n 's/.*"\(Developer ID Application:.*\)"/\1/p' | head -n 1)"
fi
[[ -n "$SIGNING_IDENTITY" ]] || { echo "No Developer ID Application identity available" >&2; exit 1; }

# Sign embedded executables first, including the universal headless runner.
while IFS= read -r -d '' candidate; do
  if file -b "$candidate" | grep -q 'Mach-O'; then
    codesign --force --options runtime --timestamp --sign "$SIGNING_IDENTITY" "$candidate"
  fi
done < <(find "$APP_PATH/Contents" -type f -print0)
codesign --force --options runtime --timestamp --sign "$SIGNING_IDENTITY" "$APP_PATH"
codesign --verify --deep --strict --verbose=2 "$APP_PATH"

mkdir -p "$(dirname "$ARCHIVE_PATH")"
python3 -c 'import sys; from pathlib import Path; Path(sys.argv[1]).unlink(missing_ok=True)' "$ARCHIVE_PATH"
ditto -c -k --keepParent "$APP_PATH" "$ARCHIVE_PATH"
xcrun notarytool submit "$ARCHIVE_PATH" --apple-id "$APPLE_ID" \
  --team-id "$APPLE_TEAM_ID" --password "$APPLE_APP_SPECIFIC_PASSWORD" --wait
xcrun stapler staple "$APP_PATH"
xcrun stapler validate "$APP_PATH"
codesign --verify --deep --strict --verbose=2 "$APP_PATH"
spctl --assess --type execute --verbose=4 "$APP_PATH"
# Recreate the archive so downloads contain the stapled bundle.
python3 -c 'import sys; from pathlib import Path; Path(sys.argv[1]).unlink(missing_ok=True)' "$ARCHIVE_PATH"
ditto -c -k --keepParent "$APP_PATH" "$ARCHIVE_PATH"
