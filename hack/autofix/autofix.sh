#!/bin/bash
# Unattended backlog fixer — see autofix.py. PLAN_PROJECT overrides (default 2).
exec python3 "$(dirname "$0")/autofix.py" "$@"
