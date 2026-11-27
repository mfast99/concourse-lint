#!/usr/bin/env bash
# Compares the test cases against real Concourse (requires fly).
#
# Must be accepted by fly (otherwise exit 1):
#   valid/, suppress/ and every rules/*/*/negatives.yml. These are clean
#   pipelines; if fly rejects one, our idea of "valid" is wrong.
# For information only:
#   all other rules/ cases and syntax/. If fly rejects a case, Concourse sees
#   the problem too. If fly accepts it, our rule goes beyond fly. That is
#   allowed (security and best-practice rules always do), but good to know.
set -u
cd "$(dirname "$0")/../spec" || exit 2

if ! command -v fly >/dev/null 2>&1; then
  echo "fly not found in PATH" >&2
  exit 2
fi

status=0
must_ok=0 must_err=0 rejected=0 accepted=0

for f in valid/*.yml suppress/*.yml rules/*/*/negatives.yml; do
  if out=$(fly validate-pipeline -c "$f" 2>&1); then
    echo "OK     $f"
    must_ok=$((must_ok + 1))
  else
    echo "ERROR  $f should be valid:"; echo "$out" | sed 's/^/         /'
    must_err=$((must_err + 1))
    status=1
  fi
done

for f in rules/*/*/*.yml syntax/*.yml; do
  [[ $(basename "$f") == negatives.yml ]] && continue
  if fly validate-pipeline -c "$f" >/dev/null 2>&1; then
    echo "INFO   $f is accepted by fly"
    accepted=$((accepted + 1))
  else
    echo "OK     $f is rejected"
    rejected=$((rejected + 1))
  fi
done

echo
echo "must be valid:  $must_ok ok, $must_err rejected by fly"
echo "other cases:    $rejected rejected by fly, $accepted accepted by fly"
exit $status
