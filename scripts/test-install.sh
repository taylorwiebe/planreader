#!/bin/sh
set -eu

root="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
sh -n "$root/install.sh"
if grep -E 'PLANREADER_(TEST|RELEASE)' "$root/install.sh" >/dev/null; then
  echo "public installer contains a trust-bypass environment override" >&2
  exit 1
fi
grep -F -- "--proto '=https'" "$root/install.sh" >/dev/null
grep -F 'codesign --verify --strict' "$root/install.sh" >/dev/null
grep -F 'TeamIdentifier=$expected_team_id' "$root/install.sh" >/dev/null
grep -F 'spctl --assess --type execute' "$root/install.sh" >/dev/null
echo "public installer trust policy passed"
