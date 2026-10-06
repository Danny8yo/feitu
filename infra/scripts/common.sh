#!/usr/bin/env bash
set -euo pipefail
INFRA_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FEITU_CONTEXT="${FEITU_CONTEXT:-kind-study-k8s}"
NAMESPACE=feitu-dev
case "$FEITU_CONTEXT" in kind-*) ;; *) echo 'Only an explicitly selected local kind context is supported.' >&2; exit 1;; esac
k() { kubectl --context "$FEITU_CONTEXT" -n "$NAMESPACE" "$@"; }
