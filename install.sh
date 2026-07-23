#!/bin/sh
set -eu

expected_team_id='__PLANREADER_TEAM_ID__'
[ "$expected_team_id" != '__PLANREADER_TEAM_ID__' ] || { echo "Use the installer attached to a Planreader release." >&2; exit 1; }

[ "$(uname -s)" = "Darwin" ] && [ "$(uname -m)" = "arm64" ] || { echo "Planreader currently supports Apple-silicon macOS only." >&2; exit 1; }

api="https://api.github.com/repos/taylorwiebe/planreader/releases/latest"
work="$(mktemp -d "${TMPDIR:-/tmp}/planreader-install.XXXXXX")"
trap 'rm -rf "$work"' EXIT HUP INT TERM
download() {
  url="$1" destination="$2" kind="$3"
  effective="$(curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --tlsv1.2 --max-filesize 268435456 --write-out '%{url_effective}' "$url" -o "$destination")"
  host="$(printf '%s\n' "$effective" | sed -n 's#^https://\([^/:]*\).*#\1#p')"
  case "$kind:$host" in
    api:api.github.com|asset:github.com|asset:objects.githubusercontent.com|asset:release-assets.githubusercontent.com) ;;
    *) echo "Download redirected to an unapproved host." >&2; exit 1 ;;
  esac
}
download "$api" "$work/release.json" api
tag="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$work/release.json" | head -n 1)"
[ -n "$tag" ] || { echo "Could not resolve the latest Planreader release." >&2; exit 1; }
printf '%s\n' "$tag" | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' >/dev/null || { echo "Latest Planreader release has an invalid version." >&2; exit 1; }
asset="planreader-${tag}-darwin-arm64.tar.gz"
base="https://github.com/taylorwiebe/planreader/releases/download/$tag"
download "$base/checksums.txt" "$work/checksums.txt" asset
download "$base/$asset" "$work/$asset" asset
(cd "$work" && grep "  $asset\$" checksums.txt > selected-checksum && shasum -a 256 -c selected-checksum)
expanded="$(gzip -dc "$work/$asset" | head -c 536870913 | wc -c | tr -d ' ')"
[ "$expanded" -le 536870912 ] || { echo "Release expands beyond the safe size limit." >&2; exit 1; }
mkdir "$work/payload"
tar -tvzf "$work/$asset" | awk '
  {
    type=substr($1,1,1); name=$NF
    if (seen[name]++) { bad=1; exit }
    if (name=="planreader" && type=="-") { binary=1; next }
    if (name=="lib/" && type=="d") next
    if (name=="lib/libsherpa-onnx-c-api.dylib" && type=="-") { sherpa=1; next }
    if (name=="lib/libonnxruntime.1.27.0.dylib" && type=="-") { onnx=1; next }
    bad=1; exit
  }
  END { exit (bad || !(binary && sherpa && onnx)) }
' || { echo "Release contains unsafe or unexpected files." >&2; exit 1; }
tar -xzf "$work/$asset" -C "$work/payload"
codesign --verify --strict --verbose=2 "$work/payload/planreader"
codesign -dv --verbose=4 "$work/payload/planreader" 2>&1 | grep -F "TeamIdentifier=$expected_team_id" >/dev/null || { echo "Release publisher identity does not match Planreader." >&2; exit 1; }
spctl --assess --type execute "$work/payload/planreader"
exec "$work/payload/planreader" install "$@"
