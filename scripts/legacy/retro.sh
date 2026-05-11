#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: retro.sh <run-dir> [work-dir]" >&2
  exit 2
fi

run_dir="$1"
work_dir="${2:-$(pwd)}"
retro_file="$run_dir/retro.md"
dashboard_file="$work_dir/.dft/runs/dashboard.md"

mkdir -p "$run_dir" "$(dirname "$dashboard_file")"

started_at="unknown"
finished_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
[[ -f "$run_dir/started_at.txt" ]] && started_at="$(cat "$run_dir/started_at.txt")"

{
  echo "# Run retrospective"
  echo
  echo "- run_dir: \`$run_dir\`"
  echo "- started_at: \`$started_at\`"
  echo "- finished_at: \`$finished_at\`"
  echo
  echo "## Stage results"
  for stage in intent design orchestration; do
    status_file="$run_dir/$stage.status"
    status="missing"
    [[ -f "$status_file" ]] && status="$(cat "$status_file")"
    echo "- $stage: **$status**"
  done
  echo
  echo "## Verification"
  if [[ -f "$run_dir/verify.log" ]]; then
    cat "$run_dir/verify.log"
  else
    echo "- No verification log found."
  fi
  echo
  echo "## Recommendations for next increment"
  echo "- Keep demand-package scope small and acceptance criteria explicit."
  echo "- Split large specs into child specs before implementation."
  echo "- Convert repeated shell logic into closed-set function steps."
} > "$retro_file"

{
  echo "# Bootstrap dashboard"
  echo
  echo "| Run | Started | Finished | Intent | Design | Orchestration |"
  echo "|---|---|---|---|---|---|"
  for r in "$work_dir"/.dft/runs/bootstrap-*; do
    [[ -d "$r" ]] || continue
    b="$(basename "$r")"
    s="unknown"
    f="unknown"
    [[ -f "$r/started_at.txt" ]] && s="$(cat "$r/started_at.txt")"
    [[ -f "$r/finished_at.txt" ]] && f="$(cat "$r/finished_at.txt")"
    i="$(cat "$r/intent.status" 2>/dev/null || echo missing)"
    d="$(cat "$r/design.status" 2>/dev/null || echo missing)"
    o="$(cat "$r/orchestration.status" 2>/dev/null || echo missing)"
    echo "| $b | $s | $f | $i | $d | $o |"
  done
} > "$dashboard_file"

echo "Wrote retrospective: $retro_file"
echo "Updated dashboard: $dashboard_file"

