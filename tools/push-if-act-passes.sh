#!/usr/bin/env bash
set -euo pipefail

# Run core CI jobs locally with `act` before pushing. If local runs succeed,
# push the current branch to origin. Intended for use only in this repository.

usage() {
  cat <<EOF
Usage: $0

Environment:
  GITHUB_TOKEN  (required) - token used by the workflow when running under act.

This script runs the following jobs from .github/workflows/ci.yml using act:
  - trivy-scan
  - test-and-build

If both jobs succeed, the script runs: git push origin HEAD

Install act: https://github.com/nektos/act
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if ! command -v act >/dev/null 2>&1; then
  echo "ERROR: 'act' is not installed. Install from https://github.com/nektos/act"
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "ERROR: working tree is not clean. Commit or stash changes before running this script."
  git status --porcelain
  exit 1
fi

if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  echo "ERROR: GITHUB_TOKEN environment variable is not set. Export it before running this script."
  echo "Example: export GITHUB_TOKEN=\$(gh auth token)"
  exit 1
fi

JOBS=(trivy-scan test-and-build)

for job in "${JOBS[@]}"; do
  echo "\n=== Running act job: $job ==="
  # Run act with the provided token as a secret. Use -j to run a single job.
  if ! act -j "$job" -s GITHUB_TOKEN="$GITHUB_TOKEN" --env CI=true --container-architecture linux/amd64; then
    echo "ERROR: act job '$job' failed. Aborting push."
    exit 1
  fi
done

echo "\nAll local act jobs passed. Pushing current branch to origin..."
git push origin HEAD
