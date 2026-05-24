#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[real-e2e] %s\n' "$*"
}

fail() {
  printf '[real-e2e] ERROR: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

assert_file() {
  [ -f "$1" ] || fail "missing file: $1"
}

assert_dir() {
  [ -d "$1" ] || fail "missing directory: $1"
}

assert_grep() {
  local pattern="$1"
  local path="$2"
  grep -Eq "$pattern" "$path" || fail "pattern '$pattern' not found in $path"
}

timestamp="$(date -u +%Y%m%d%H%M%S)"
owner="${DFT_E2E_OWNER:-bocacorazon}"
repo_name="${DFT_E2E_REPO_NAME:-dft-real-e2e}"
repo="${owner}/${repo_name}"
run_id="${DFT_RUN_ID:-real-e2e-${timestamp}}"
test_branch="${DFT_E2E_BRANCH:-e2e/${run_id}}"
work_root="${DFT_E2E_WORK_ROOT:-$(mktemp -d "${TMPDIR:-/tmp}/dft-real-e2e.XXXXXX")}"
local_repo="${work_root}/${repo_name}"
results_dir="${DFT_E2E_RESULTS_DIR:-${work_root}/results/${run_id}}"
agent_timeout="${DFT_E2E_AGENT_TIMEOUT:-45m}"
eval_retries="${DFT_E2E_EVAL_RETRIES:-1}"
copilot_binary="${DFT_E2E_COPILOT_BINARY:-copilot}"
source_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dft_bin="${DFT_E2E_DFT_BIN:-${work_root}/bin/dft}"
hold_arg=("--hold-increment")

if [ "${DFT_E2E_FINAL_MERGE:-0}" = "1" ]; then
  hold_arg=()
fi

demand="${DFT_E2E_DEMAND:-Build a minimal Go CLI named democtl. It must have a go.mod, a command under cmd/democtl, and support --version. Running the built binary with --version must exit 0 and print exactly 0.1.0. Keep the implementation standard-library only. Acceptance criteria: go test ./... passes; go build -o bin/democtl ./cmd/democtl succeeds; bin/democtl --version exits 0 and trimmed stdout equals 0.1.0.}"

require_command gh
require_command git
require_command go
require_command grep
require_command cp
command -v "$copilot_binary" >/dev/null 2>&1 || fail "real Copilot binary not found: $copilot_binary"

gh auth status >/dev/null || fail "gh auth status failed"

mkdir -p "$(dirname "$dft_bin")" "$results_dir"
log "building dft from ${source_root}"
(cd "$source_root" && go build -o "$dft_bin" ./cmd/dft)

if gh repo view "$repo" >/dev/null 2>&1; then
  log "using existing private test repo ${repo}"
else
  log "creating private GitHub repo ${repo}"
  gh repo create "$repo" --private --description "Real dft end-to-end test repository"
fi

log "cloning ${repo} into ${local_repo}"
gh repo clone "$repo" "$local_repo"
cd "$local_repo"

git config user.email "${DFT_E2E_GIT_EMAIL:-dft-real-e2e@example.test}"
git config user.name "${DFT_E2E_GIT_NAME:-dft real e2e}"

if git rev-parse --verify HEAD >/dev/null 2>&1; then
  default_branch="$(git branch --show-current)"
  if [ -z "$default_branch" ]; then
    default_branch="main"
    git switch "$default_branch"
  fi
else
  default_branch="main"
  git switch -c "$default_branch"
  printf '# dft real e2e\n' > README.md
  git add README.md
  git commit -m "initial real e2e repo"
fi

log "provisioning dft assets on ${default_branch}"
if [ -f .dft/provisioning-manifest.json ]; then
  printf 'dft init skipped; existing provisioning manifest found\n' | tee "${results_dir}/dft-init.log"
else
  "$dft_bin" init | tee "${results_dir}/dft-init.log"
fi
"$dft_bin" sync --force | tee "${results_dir}/dft-sync.log"
if grep -Eq 'init-go-cli|internal/cli/version.go' .dft/flows/spec-lane.yaml; then
  fail "provisioned spec lane still contains stale feature-specific prompt text"
fi
git add .
if ! git diff --cached --quiet; then
  git commit -m "provision dft assets"
fi
git push -u origin "$default_branch"
git remote set-head origin -a >/dev/null 2>&1 || true

if git show-ref --verify --quiet "refs/heads/${test_branch}"; then
  fail "local branch already exists: ${test_branch}"
fi
if git ls-remote --exit-code --heads origin "$test_branch" >/dev/null 2>&1; then
  fail "remote branch already exists: ${test_branch}"
fi
git switch -c "$test_branch" "$default_branch"
git push -u origin "$test_branch"

submit_args=(
  submit
  --adapter copilot
  --copilot-binary "$copilot_binary"
  --full
  --agent-timeout "$agent_timeout"
  --eval-retries "$eval_retries"
)
submit_args+=("${hold_arg[@]}")
submit_args+=("$demand")

log "running real dft process: repo=${repo} branch=${test_branch} run=${run_id}"
DFT_RUN_ID="$run_id" "$dft_bin" "${submit_args[@]}" 2>&1 | tee "${results_dir}/submit.log"

"$dft_bin" status | tee "${results_dir}/status.txt"
"$dft_bin" inspect "$run_id" | tee "${results_dir}/inspect.txt"

run_dir=".dft/runs/${run_id}"
assert_dir "$run_dir"
for artifact in \
  "intent/demand-package.json" \
  "design/wbs.json" \
  "design/lane-assignments.json" \
  "design/eval-surfaces.json" \
  "eval/eval-ready.json" \
  "eval/eval-plan.json" \
  "eval/evaluation.json" \
  "review/final-review.json" \
  "macro-result.json"; do
  assert_file "${run_dir}/${artifact}"
done
assert_dir "${run_dir}/eval/evidence"
assert_grep '"status"[[:space:]]*:[[:space:]]*"pass"' "${run_dir}/eval/eval-ready.json"
assert_grep '"status"[[:space:]]*:[[:space:]]*"pass"' "${run_dir}/eval/evaluation.json"
assert_grep 'democtl|bin/democtl|cmd/democtl' "${run_dir}/design/eval-surfaces.json"
assert_grep 'democtl|bin/democtl|cmd/democtl' "${run_dir}/eval/eval-plan.json"

tasks_count="$(find ".dft/worktrees/${run_id}" -path '*/tasks.md' -type f 2>/dev/null | wc -l | tr -d ' ')"
[ "$tasks_count" != "0" ] || fail "no Speckit tasks.md files found under .dft/worktrees/${run_id}"
while IFS= read -r tasks_file; do
  if grep -Eq '^- \[ \]' "$tasks_file"; then
    fail "unchecked task remains in ${tasks_file}"
  fi
  assert_grep '^- \[[xX]\]' "$tasks_file"
done < <(find ".dft/worktrees/${run_id}" -path '*/tasks.md' -type f)

if [ "${DFT_E2E_FINAL_MERGE:-0}" = "1" ]; then
  git switch "$default_branch"
  validation_repo="$PWD"
else
  assert_grep '"increment_held"[[:space:]]*:[[:space:]]*true' "${run_dir}/macro-result.json"
  increment_ref="refs/heads/increment/${run_id}"
  validation_repo="$(git worktree list --porcelain | awk -v ref="$increment_ref" '
    /^worktree / { current = substr($0, 10) }
    /^branch / && substr($0, 8) == ref { print current; exit }
  ')"
  if [ -z "$validation_repo" ]; then
    git switch "increment/${run_id}"
    validation_repo="$PWD"
  fi
fi

log "validating generated software in ${validation_repo}"
(cd "$validation_repo" && go test ./...) | tee "${results_dir}/generated-go-test.log"
(cd "$validation_repo" && mkdir -p bin && go build -o bin/democtl ./cmd/democtl)
version_output="$("$validation_repo/bin/democtl" --version | tr -d '\r' | sed 's/[[:space:]]*$//')"
[ "$version_output" = "0.1.0" ] || fail "democtl --version printed ${version_output}, want 0.1.0"

cp -R "$run_dir" "${results_dir}/run-artifacts"
git branch --list | tee "${results_dir}/branches.txt"
git push origin "HEAD:${test_branch}"
if git show-ref --verify --quiet "refs/heads/increment/${run_id}"; then
  git push origin "increment/${run_id}"
fi
while IFS= read -r branch; do
  [ -z "$branch" ] || git push origin "$branch"
done < <(git for-each-ref --format='%(refname:short)' "refs/heads/spec/${run_id}")

cat > "${results_dir}/summary.txt" <<SUMMARY
repo: https://github.com/${repo}
test_branch: ${test_branch}
run_id: ${run_id}
local_repo: ${local_repo}
results_dir: ${results_dir}
verdict: pass
SUMMARY

cat "${results_dir}/summary.txt"
