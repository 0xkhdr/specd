#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

# Post-v0.1.0-cleanup, there is no more grace-period deprecation model in the
# codebase (commands are either present or fully deleted), so the only check
# left here is that the cheat sheet and its canonical spec copy agree.
python3 - <<'PY'
import re, sys
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
