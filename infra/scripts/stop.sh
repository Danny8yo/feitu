#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
# Scale down only this project's workloads; PVCs remain intact.
k scale statefulset mysql kafka redis --replicas=0
