#!/bin/sh
set -eu

version="${VERSION:?set VERSION to the release tag, for example v1.2.3}"
team_id="${APPLE_TEAM_ID:-TEST}"
[ "$(uname -s)" = "Darwin" ] && [ "$(uname -m)" = "arm64" ] || { echo "Packaging requires Apple-silicon macOS." >&2; exit 1; }
if [ "${PLANREADER_TEST_UNSIGNED:-}" != "1" ]; then
  : "${APPLE_DEVELOPER_ID_APPLICATION:?missing signing identity}"
  : "${APPLE_TEAM_ID:?missing Apple Team ID}"
  : "${APPLE_ID:?missing notarization Apple ID}"
  : "${APPLE_APP_PASSWORD:?missing notarization app password}"
fi

root="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
dist="$root/dist/$version"
payload="$dist/payload"
mkdir -p "$payload/lib"
commit="$(git -C "$root" rev-parse HEAD)"
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -mod=vendor -trimpath -ldflags "-X github.com/taylorwiebe/planreader/internal/buildinfo.Version=$version -X github.com/taylorwiebe/planreader/internal/buildinfo.Commit=$commit -X github.com/taylorwiebe/planreader/internal/buildinfo.Origin=release -X github.com/taylorwiebe/planreader/internal/buildinfo.TeamID=${APPLE_TEAM_ID:-TEST}" -o "$payload/planreader" "$root"
library_root="$root/vendor/github.com/k2-fsa/sherpa-onnx-go-macos/lib/aarch64-apple-darwin"
cp "$library_root/libsherpa-onnx-c-api.dylib" "$payload/lib/"
cp "$library_root/libonnxruntime.1.27.0.dylib" "$payload/lib/"
install_name_tool -add_rpath @loader_path/lib "$payload/planreader" 2>/dev/null || install_name_tool -rpath "$root/vendor/github.com/k2-fsa/sherpa-onnx-go-macos/lib/aarch64-apple-darwin" @loader_path/lib "$payload/planreader"
for library in "$payload"/lib/*.dylib; do install_name_tool -id "@rpath/$(basename "$library")" "$library"; done
otool -l "$payload/planreader" | grep -F "$root" && { echo "Repository rpath remains in packaged binary." >&2; exit 1; } || true
if [ "${PLANREADER_TEST_UNSIGNED:-}" != "1" ]; then
  for library in "$payload"/lib/*.dylib; do codesign --force --timestamp --options runtime --sign "$APPLE_DEVELOPER_ID_APPLICATION" "$library"; done
  codesign --force --timestamp --options runtime --sign "$APPLE_DEVELOPER_ID_APPLICATION" "$payload/planreader"
fi
archive="$dist/planreader-${version}-darwin-arm64.tar.gz"
(cd "$payload" && COPYFILE_DISABLE=1 tar -czf "$archive" planreader lib)
(cd "$dist" && shasum -a 256 "$(basename "$archive")" > checksums.txt)
sed "s/__PLANREADER_TEAM_ID__/$team_id/g" "$root/install.sh" > "$dist/install.sh"
if [ "${PLANREADER_TEST_UNSIGNED:-}" != "1" ]; then
  notarization_zip="$dist/planreader-${version}-notarization.zip"
  ditto -c -k --keepParent "$payload" "$notarization_zip"
  xcrun notarytool submit "$notarization_zip" --apple-id "$APPLE_ID" --password "$APPLE_APP_PASSWORD" --team-id "$APPLE_TEAM_ID" --wait
  rm "$notarization_zip"
fi
printf '%s\n' "$archive"
