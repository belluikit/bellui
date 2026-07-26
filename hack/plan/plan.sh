#!/bin/bash
# Live bellui-backlog renderer — see plan.py. PLAN_PROJECT overrides (default 2).
exec python3 "$(dirname "$0")/plan.py" "$@"
