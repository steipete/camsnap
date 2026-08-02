#!/bin/bash
set -euo pipefail

binary=${1:?usage: sign-darwin.sh <binary>}
if [[ ! -f "$binary" ]]; then
  echo "sign-darwin: binary not found: $binary" >&2
  exit 1
fi

if [[ $(file -b "$binary") != *Mach-O* ]]; then
  exit 0
fi

if ! otool -s __TEXT __info_plist "$binary" >/dev/null 2>&1; then
  echo "sign-darwin: missing __TEXT,__info_plist section: $binary" >&2
  exit 1
fi

sign_values=0
[[ -n ${MACOS_SIGN_P12_BASE64:-} ]] && sign_values=$((sign_values + 1))
[[ -n ${MACOS_SIGN_P12_PASSWORD:-} ]] && sign_values=$((sign_values + 1))
[[ -n ${MACOS_SIGN_IDENTITY:-} ]] && sign_values=$((sign_values + 1))

if [[ $sign_values -ne 0 && $sign_values -ne 3 ]]; then
  echo "sign-darwin: set MACOS_SIGN_P12_BASE64, MACOS_SIGN_P12_PASSWORD, and MACOS_SIGN_IDENTITY together" >&2
  exit 1
fi

if [[ $sign_values -eq 0 ]]; then
  codesign --force -s - "$binary"
  codesign --verify --verbose "$binary"
  echo "Signed $binary with an ad-hoc identity"
  exit 0
fi

sign_dir=$(mktemp -d)
certificate="$sign_dir/certificate.p12"
keychain="$sign_dir/camsnap-signing.keychain-db"
keychain_password=$(uuidgen)
cleanup() {
  security delete-keychain "$keychain" >/dev/null 2>&1 || true
  rm -rf "$sign_dir"
}
trap cleanup EXIT

printf '%s' "$MACOS_SIGN_P12_BASE64" | base64 -D >"$certificate"
security create-keychain -p "$keychain_password" "$keychain"
security set-keychain-settings -lut 21600 "$keychain"
security unlock-keychain -p "$keychain_password" "$keychain"
security import "$certificate" -k "$keychain" -P "$MACOS_SIGN_P12_PASSWORD" -T /usr/bin/codesign
security set-key-partition-list -S apple-tool:,apple: -s -k "$keychain_password" "$keychain" >/dev/null

codesign --force --options runtime --timestamp --keychain "$keychain" -s "$MACOS_SIGN_IDENTITY" "$binary"
codesign --verify --verbose "$binary"
echo "Signed $binary with $MACOS_SIGN_IDENTITY"

# Notarize before anything is published: this hook runs during the goreleaser
# build phase, so a notarization rejection fails the release while all
# artifacts are still local.
notarize_values=0
[[ -n ${APP_STORE_CONNECT_KEY_ID:-} ]] && notarize_values=$((notarize_values + 1))
[[ -n ${APP_STORE_CONNECT_ISSUER_ID:-} ]] && notarize_values=$((notarize_values + 1))
[[ -n ${APP_STORE_CONNECT_API_KEY_P8:-} ]] && notarize_values=$((notarize_values + 1))

if [[ $notarize_values -ne 0 && $notarize_values -ne 3 ]]; then
  echo "sign-darwin: set APP_STORE_CONNECT_KEY_ID, APP_STORE_CONNECT_ISSUER_ID, and APP_STORE_CONNECT_API_KEY_P8 together" >&2
  exit 1
fi

if [[ $notarize_values -eq 3 ]]; then
  key_file="$sign_dir/AuthKey_${APP_STORE_CONNECT_KEY_ID}.p8"
  printf '%s' "$APP_STORE_CONNECT_API_KEY_P8" >"$key_file"
  chmod 600 "$key_file"
  archive="$sign_dir/$(basename "$binary").zip"
  ditto -c -k --keepParent "$binary" "$archive"
  xcrun notarytool submit "$archive" --wait \
    --key "$key_file" \
    --key-id "$APP_STORE_CONNECT_KEY_ID" \
    --issuer "$APP_STORE_CONNECT_ISSUER_ID"
  rm -f "$key_file" "$archive"
  echo "Notarized $binary"
fi
