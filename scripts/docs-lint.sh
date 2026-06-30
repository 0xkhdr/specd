#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

csv=".specd/specs/cmd-audit/audit.csv"
if [[ ! -f "$csv" ]]; then
  echo "missing $csv" >&2
  exit 1
fi

mapfile -t dead < <(awk -F, 'NR>1 && ($10=="merge" || $10=="deprecate") {print $1}' "$csv" | sort -u)

files=(README.md AGENTS.md)
while IFS= read -r -d '' f; do
  files+=("$f")
done < <(find docs -maxdepth 1 -type f -name '*.md' -print0 | sort -z)

fail=0
for file in "${files[@]}"; do
  in_appendix=0
  line_no=0
  while IFS= read -r line || [[ -n "$line" ]]; do
    line_no=$((line_no + 1))
    if [[ "$file" == "docs/command-reference.md" ]]; then
      [[ "$line" == *"docs-lint: migration-appendix begin"* ]] && in_appendix=1 && continue
      [[ "$line" == *"docs-lint: migration-appendix end"* ]] && in_appendix=0 && continue
      [[ $in_appendix -eq 1 ]] && continue
    fi
    for cmd in "${dead[@]}"; do
      if [[ "$line" =~ (^|[^[:alnum:]_-])specd[[:space:]]+${cmd}($|[^[:alnum:]_-]) ]] || \
         [[ "$line" =~ (^|[^[:alnum:]_-])specd_${cmd}($|[^[:alnum:]_-]) ]]; then
        printf '%s:%d: dead command reference: %s\n' "$file" "$line_no" "$cmd" >&2
        fail=1
      fi
    done
  done < "$file"
done

python3 - <<'PY'
import re, subprocess, sys
from pathlib import Path
survivors=['init','new','status','context','check','approve','next','verify','task','report','decision','midreq','memory','waves','brain','pinky','version','help','mcp','fusion']
text=Path('docs/command-reference.md').read_text()
listed=re.findall(r'^\| `specd ([a-z-]+)` \|', text, re.M)
# First table is cheat sheet, subsequent command tables duplicate; assert cheat sheet exactly.
cheat=listed[:20]
if cheat != survivors:
    print(f'command-reference cheat sheet mismatch: {cheat}', file=sys.stderr)
    sys.exit(1)
cs=Path('.specd/specs/CHEATSHEET.md').read_text()
count=len(re.findall(r'^\| `[^`]+` \|', cs, re.M))
if count != 20:
    print(f'CHEATSHEET count {count}, want 20', file=sys.stderr)
    sys.exit(1)
PY

exit "$fail"
