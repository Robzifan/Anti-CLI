#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
SKIP = {'.git', 'bin', 'dist'}
PATTERNS = [
    re.compile(r'(?i)(token_v2|api[_-]?key|authorization|cookie)\s*[:=]\s*["\']?(?!<|\[REDACTED\]|redacted|forbidden|secret\b)[A-Za-z0-9_./+\-=]{12,}'),
    re.compile(r'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'),
]
failed = False
for path in ROOT.rglob('*'):
    if not path.is_file() or any(part in SKIP for part in path.parts):
        continue
    try:
        text = path.read_text(encoding='utf-8')
    except UnicodeDecodeError:
        continue
    for number, line in enumerate(text.splitlines(), 1):
        if any(pattern.search(line) for pattern in PATTERNS):
            print(f'{path.relative_to(ROOT)}:{number}: possible secret')
            failed = True
sys.exit(1 if failed else 0)
