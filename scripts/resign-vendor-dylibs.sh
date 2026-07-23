#!/bin/sh
# Upstream sherpa-onnx modules ship dylibs whose ad-hoc signatures no longer
# match their contents. macOS registers the bad signature when the dylib is
# loaded and then kills any process that maps the file — including git while
# hashing the working tree. Run this after `go mod vendor` to make the
# vendored copies validly signed. Safe to run repeatedly.
set -eu
cd "$(dirname "$0")/.."
find vendor -name '*.dylib' | while read -r library; do
  directory=$(dirname "$library")
  chmod u+w "$directory" "$library"
  codesign --force --sign - "$library"
  chmod u-w "$directory" "$library"
done
