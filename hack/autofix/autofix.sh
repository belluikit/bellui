#!/bin/bash
# Unattended backlog fixer — see autofix.py. Optional config.json/PROTOCOL.md
# beside it specialize per repo.
exec python3 "$(dirname "$0")/autofix.py" "$@"
