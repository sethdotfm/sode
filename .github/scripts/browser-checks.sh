#!/usr/bin/env bash
# Usage: browser-checks.sh <shard> <shards> <-run regex> [-skip regex]
# Runs this shard's share of ./internal/app tests matching the regex (split by name with -list, so
# no test code knows about shards), then reruns each failure once. A test that passes on the rerun
# is reported as flaky and does not fail the job; see AGENTS.md "Test groups".
set -uo pipefail
shard=$1 shards=$2 run=$3 skip=${4:-}

# No mapfile: macOS still ships bash 3.2.
mine=() i=0
while IFS= read -r name; do
  if (( i % shards == shard )); then mine+=("$name"); fi
  i=$((i + 1))
done < <(go test -list "$run" ./internal/app | grep '^Test' | { if [[ -n $skip ]]; then grep -Ev "$skip"; else cat; fi; } | sort)
if (( ${#mine[@]} == 0 )); then
  echo "::error::shard $shard of $shards matched no tests for $run"
  exit 1
fi
printf 'shard %s/%s runs %s tests:\n' "$shard" "$shards" "${#mine[@]}"
printf '  %s\n' "${mine[@]}"
pattern="^($(IFS='|'; echo "${mine[*]}"))\$"

out=$(mktemp)
go test -count=1 -timeout 20m -json -run "$pattern" ./internal/app >"$out"
code=$?
jq -r 'select(.Action=="output") | .Output' "$out" | grep -E '^(---|=== RUN|ok|FAIL|panic)|_test\.go:[0-9]+:' || true
(( code == 0 )) && exit 0

failed=()
while IFS= read -r name; do failed+=("$name"); done < <(jq -r 'select(.Action=="fail" and .Test != null) | .Test | split("/")[0]' "$out" | sort -u)
if (( ${#failed[@]} == 0 )); then
  echo "::error::go test failed with no failing test (build error, panic or timeout)"
  jq -r 'select(.Action=="output") | .Output' "$out" | tail -80
  exit 1
fi
echo "rerunning once: ${failed[*]}"
if go test -count=1 -timeout 20m -v -run "^($(IFS='|'; echo "${failed[*]}"))\$" ./internal/app; then
  echo "::warning::flaky (failed, then passed on rerun): ${failed[*]}"
  exit 0
fi
exit 1
