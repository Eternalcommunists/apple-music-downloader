#!/usr/bin/env bash
# Sanity checks for scripts/build.sh argument handling and error paths.
# Run from the repository root: bash scripts/build.sh.selftest.sh
export PATH=/usr/bin:/bin
cd /mnt/d/apple-music-downloader || exit 1

pass=0
fail=0
check() {
  local desc="$1" want="$2" got="$3"
  if [ "$want" = "$got" ]; then
    echo "  PASS  $desc"
    pass=$((pass + 1))
  else
    echo "  FAIL  $desc (want $want, got $got)"
    fail=$((fail + 1))
  fi
}

echo "--- bad target should be rejected before any build ---"
./scripts/build.sh --targets bogus > /tmp/st.out 2>/tmp/st.err
check "single segment rejected" "2" "$?"
grep -q "expected os/arch" /tmp/st.err && echo "  PASS  error mentions os/arch" || echo "  FAIL  missing hint"

echo "--- multi-slash target rejected ---"
./scripts/build.sh --targets a/b/c > /tmp/st.out 2>/tmp/st.err
check "a/b/c rejected" "2" "$?"

echo "--- empty arch rejected ---"
./scripts/build.sh --targets linux/ > /tmp/st.out 2>/tmp/st.err
check "linux/ rejected" "2" "$?"

echo "--- unknown argument rejected ---"
./scripts/build.sh --nope > /tmp/st.out 2>/tmp/st.err
check "unknown flag rejected" "2" "$?"

echo "--- --help exits 0 and prints usage without the shebang body ---"
./scripts/build.sh --help > /tmp/st.help 2>&1
check "--help exit 0" "0" "$?"
grep -q "^Usage:" /tmp/st.help && echo "  PASS  prints Usage" || echo "  FAIL  no Usage"
if grep -q "set -euo pipefail" /tmp/st.help; then
  echo "  FAIL  help leaks script body"
else
  echo "  PASS  help does not leak script body"
fi

echo "--- version normalisation: 0.9.9 must be tagged v0.9.9 ---"
# Build a bogus target so the run stops before compiling, and read the banner.
./scripts/build.sh --version 0.9.9 --targets bogus > /tmp/st.out 2>&1
grep -q "version    : v0.9.9" /tmp/st.out && echo "  PASS  adds v prefix" || echo "  FAIL  did not add v prefix"
./scripts/build.sh --version v9.9.9 --targets bogus > /tmp/st.out 2>&1
grep -q "version    : v9.9.9" /tmp/st.out && echo "  PASS  does not double the v" || echo "  FAIL  doubled the v"

echo
echo "passed=$pass failed=$fail"
exit "$fail"