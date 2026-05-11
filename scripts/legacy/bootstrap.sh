#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  bootstrap.sh --feature-file <path> [--work-dir <dir>] [--run-id <id>]

Options:
  --feature-file   File containing feature description text (required)
  --work-dir       Target repo path (default: current directory)
  --run-id         Override run id (default: bootstrap-YYYYmmdd-HHMMSS)
EOF
}

feature_file=""
work_dir="$(pwd)"
run_id="bootstrap-$(date +%Y%m%d-%H%M%S)"
copilot_flags="${COPILOT_FLAGS:---allow-all -s --no-ask-user --autopilot}"
agent_timeout_seconds="${AGENT_TIMEOUT_SECONDS:-420}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --feature-file)
      feature_file="$2"
      shift 2
      ;;
    --work-dir)
      work_dir="$2"
      shift 2
      ;;
    --run-id)
      run_id="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$feature_file" ]]; then
  echo "Missing --feature-file" >&2
  usage
  exit 2
fi

if [[ ! -f "$feature_file" ]]; then
  echo "Feature file not found: $feature_file" >&2
  exit 2
fi

command -v copilot >/dev/null
command -v git >/dev/null

run_dir="$work_dir/.dft/runs/$run_id"
mkdir -p "$run_dir"
start_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "$start_utc" > "$run_dir/started_at.txt"

feature_text="$(cat "$feature_file")"
verdict_file="$work_dir/.pipeline/runtime/verdict-failures.md"

run_agent() {
  local stage="$1"
  local agent="$2"
  local prompt="$3"
  local out_file="$4"
  local safe_agent="${agent//./_}"
  local agent_status_file="$run_dir/$stage.$safe_agent.status"
  echo "=== $stage :: $agent ==="
  if timeout "${agent_timeout_seconds}s" copilot -C "$work_dir" -p "$prompt" --agent "$agent" $copilot_flags > "$out_file" 2> "$run_dir/$stage.$agent.stderr"; then
    echo "ok" > "$agent_status_file"
    echo "ok" > "$run_dir/$stage.status"
    return 0
  fi
  local ec=$?
  if [[ $ec -eq 124 && -s "$out_file" ]]; then
    echo "timed_out_with_output" > "$agent_status_file"
    echo "ok" > "$run_dir/$stage.status"
    return 0
  fi
  if [[ -s "$out_file" ]]; then
    echo "nonzero_with_output" > "$agent_status_file"
    echo "ok" > "$run_dir/$stage.status"
    return 0
  fi
  echo "failed" > "$agent_status_file"
  echo "failed" > "$run_dir/$stage.status"
  return 1
}

verify() {
  ./scripts/legacy/verify-checks.sh "$@" >> "$run_dir/verify.log"
}

pushd "$work_dir" >/dev/null

# intent
run_agent "intent" "dft.demand-package" \
  "Create demand-package YAML only from this feature request: $feature_text" \
  "$run_dir/demand-package.yaml"
verify file_exists "$run_dir/demand-package.yaml"

# design
run_agent "design" "dft.wbs-builder" \
  "Convert this demand-package to WBS YAML only: $(cat "$run_dir/demand-package.yaml")" \
  "$run_dir/wbs.yaml"
verify file_exists "$run_dir/wbs.yaml"

run_agent "design" "dft.lane-selector" \
  "Assign lanes for this WBS as YAML only: $(cat "$run_dir/wbs.yaml")" \
  "$run_dir/lanes.yaml"
verify file_exists "$run_dir/lanes.yaml"

# orchestration (speckit baseline loop)
run_agent "orchestration" "speckit.specify" \
  "Run speckit specify for this feature. Complete and exit without waiting for follow-up input: $feature_text" \
  "$run_dir/specify.out.md"
run_agent "orchestration" "speckit.plan" \
  "Run only the planning step for the current feature branch. Do not trigger handoffs. Complete and exit." \
  "$run_dir/plan.out.md"
run_agent "orchestration" "speckit.tasks" \
  "Run only task generation for the current feature branch. Do not trigger handoffs. Complete and exit." \
  "$run_dir/tasks.out.md"
implement_prompt="Run implementation for current feature branch. If checklists are incomplete, continue without asking for confirmation. Complete and exit."
if [[ -f "$verdict_file" ]]; then
  implement_prompt="Read .pipeline/runtime/verdict-failures.md first and focus only on fixing listed failing criteria plus directly related gaps. Do not redo already-satisfied behavior. Then run implementation for current feature branch and exit."
fi
run_agent "orchestration" "speckit.implement" "$implement_prompt" "$run_dir/implement.out.md"

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ -f "specs/$branch/spec.md" ]]; then
  verify file_exists "specs/$branch/spec.md"
fi
if [[ -f "specs/$branch/plan.md" ]]; then
  verify file_exists "specs/$branch/plan.md"
fi
if [[ -f "specs/$branch/tasks.md" ]]; then
  verify file_exists "specs/$branch/tasks.md"
fi

popd >/dev/null

echo "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$run_dir/finished_at.txt"
./scripts/legacy/retro.sh "$run_dir" "$work_dir"

echo "Bootstrap run completed: $run_dir"
