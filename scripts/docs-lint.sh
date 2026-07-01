#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

# Post-v0.1.0-cleanup, there is no more grace-period deprecation model in the
# codebase (commands are either present or fully deleted), so the only check
# left here is that the cheat sheet and its canonical spec copy agree.
#
# `docs/command-reference.md`'s "## Cheat sheet" table is the single source of
# truth; `.specd/specs/CHEATSHEET.md` is a verbatim mirror. This lint asserts
# *content equality* between the two tables (not just a row count) so the mirror
# cannot silently drift — a wrong command, reordering, or edited description in
# either file fails the check. The survivor list is derived from the source
# table, not hardcoded here, so adding/removing a command only requires editing
# the two doc tables (which this check then keeps in lockstep).
python3 - <<'PY'
import re, sys
from pathlib import Path

ROW = re.compile(r'^\| (`[^`]+`) \| (.+?) \|\s*$', re.M)

def cheat_rows(text, *, section=None):
    """Return [(command, description)] for the first markdown table.

    If `section` is given, scope to the block under that "## <section>" heading
    up to the next "## " heading; otherwise scan the whole document (used for
    CHEATSHEET.md, which is a single table)."""
    if section is not None:
        m = re.search(rf'^##\s+{re.escape(section)}\s*$(.*?)(?=^##\s|\Z)',
                      text, re.M | re.S)
        if not m:
            sys.exit(f'section "{section}" not found')
        text = m.group(1)
    rows = ROW.findall(text)
    # Drop a leading header row like "| Command | ... |" if present.
    return [(c, d.strip()) for c, d in rows if c not in ('Command', '`Command`')]

ref = cheat_rows(Path('docs/command-reference.md').read_text(), section='Cheat sheet')
mirror = cheat_rows(Path('.specd/specs/CHEATSHEET.md').read_text())

if not ref:
    sys.exit('command-reference cheat sheet is empty')

if ref != mirror:
    # Report the first divergence for a fast fix.
    for i, (a, b) in enumerate(zip(ref, mirror)):
        if a != b:
            print(f'cheat-sheet drift at row {i + 1}:', file=sys.stderr)
            print(f'  command-reference: {a}', file=sys.stderr)
            print(f'  CHEATSHEET.md:     {b}', file=sys.stderr)
            break
    else:
        print(f'cheat-sheet row-count drift: command-reference has {len(ref)}, '
              f'CHEATSHEET.md has {len(mirror)}', file=sys.stderr)
    sys.exit(1)

print(f'docs-lint ok: cheat sheet mirrors match ({len(ref)} commands)')
PY
