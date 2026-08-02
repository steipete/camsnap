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
  rm -f "$certificate"
  rmdir "$sign_dir" 2>/dev/null || true
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
