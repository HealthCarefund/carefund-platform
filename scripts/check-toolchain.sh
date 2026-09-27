#!/usr/bin/env bash
# Verifies that the tool versions pinned in TOOLCHAIN.md are actually
# selected in the current shell. Exits non-zero on the first mismatch.
set -euo pipefail

fail=0

check() {
  local name="$1" expected="$2" actual="$3"
  if [[ "$actual" != *"$expected"* ]]; then
    echo "MISMATCH: $name expected '$expected', got '$actual'" >&2
    fail=1
  else
    echo "OK: $name -> $actual"
  fi
}

check "Go" "go1.27.1" "$(go version)"
check "Node" "v24.21.0" "$(node --version)"
check "pnpm" "12.5.1" "$(pnpm --version)"
check "Rust" "1.98.1" "$(rustc --version)"
check "Stellar CLI" "27.0.0" "$(stellar --version | head -n1)"

pg_version=$(PGPASSWORD=carefund_dev psql -h 127.0.0.1 -p 5439 -U carefund -d carefund -tAc "SELECT version();" 2>&1) || {
  echo "MISMATCH: PostgreSQL could not connect to isolated instance on port 5439: $pg_version" >&2
  fail=1
}
if [[ "${pg_version:-}" == *"18.6"* ]]; then
  echo "OK: PostgreSQL -> $pg_version"
elif [[ -n "${pg_version:-}" ]]; then
  echo "MISMATCH: PostgreSQL expected '18.6', got '$pg_version'" >&2
  fail=1
fi

exit $fail
