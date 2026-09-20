#!/usr/bin/env bash
# Runs the invoice benchmarks n times for each implementation and compares them
# with benchstat, which reports the delta with a p-value rather than a single
# lucky run. Results land in results/.
set -euo pipefail

cd "$(dirname "$0")/.."

COUNT="${COUNT:-10}"
BENCHSTAT="$(command -v benchstat || echo "$(go env GOPATH)/bin/benchstat")"

if [ ! -x "$BENCHSTAT" ]; then
    echo "benchstat not found. Install it with:" >&2
    echo "  go install golang.org/x/perf/cmd/benchstat@latest" >&2
    exit 1
fi

mkdir -p results

echo "Running baseline benchmarks (n=$COUNT)..."
go test ./internal/invoice/ -run '^$' -bench 'Baseline' -count="$COUNT" \
    | sed 's/_Baseline_/_/' > results/baseline-bench.txt

echo "Running batched benchmarks (n=$COUNT)..."
go test ./internal/invoice/ -run '^$' -bench 'Batched' -count="$COUNT" \
    | sed 's/_Batched_/_/' > results/improved-bench.txt

echo
"$BENCHSTAT" results/baseline-bench.txt results/improved-bench.txt \
    | tee results/benchstat.txt
