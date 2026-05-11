#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  verify-checks.sh file_exists <path>
  verify-checks.sh command_exit_zero <command> [args...]
EOF
}

if [[ $# -lt 2 ]]; then
  usage
  exit 2
fi

check="$1"
shift

case "$check" in
  file_exists)
    target="$1"
    if [[ -f "$target" ]]; then
      echo "PASS file_exists $target"
      exit 0
    fi
    echo "FAIL file_exists $target"
    exit 1
    ;;
  command_exit_zero)
    if "$@"; then
      echo "PASS command_exit_zero $*"
      exit 0
    fi
    echo "FAIL command_exit_zero $*"
    exit 1
    ;;
  *)
    echo "Unknown check: $check"
    usage
    exit 2
    ;;
esac

