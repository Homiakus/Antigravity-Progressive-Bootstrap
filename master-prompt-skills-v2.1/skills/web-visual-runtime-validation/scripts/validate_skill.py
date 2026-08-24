#!/usr/bin/env python3
from pathlib import Path
import re,sys
root=Path(__file__).resolve().parents[1]
text=(root/"SKILL.md").read_text(encoding="utf-8")
errors=[]
m=re.match(r"^---\n(.*?)\n---\n(.*)$",text,re.S)
if not m: errors.append("missing YAML frontmatter")
else:
 fm,body=m.groups()
 nm=re.search(r"^name:\s*(.+)$",fm,re.M)
 if not nm: errors.append("missing name")
 else:
  n=nm.group(1).strip()
  if n!=root.name: errors.append(f"name {n!r} != directory {root.name!r}")
  if len(n)>64 or not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*",n): errors.append("invalid name format")
 dm=re.search(r"^description:\s*>-\n((?:  .*\n?)+)",fm,re.M)
 if not dm: errors.append("missing folded description")
 else:
  desc=' '.join(x.strip() for x in dm.group(1).splitlines())
  if not (1<=len(desc)<=1024): errors.append(f"description length {len(desc)}")
 if len(body.splitlines())>500: errors.append(f"SKILL.md body over recommended 500 lines: {len(body.splitlines())}")
 for rel in re.findall(r"`((?:references|scripts|assets)/[^`]+)`",body):
  if not (root/rel).exists(): errors.append(f"missing reference: {rel}")
if errors:
 print("INVALID",root.name)
 [print(" -",e) for e in errors]
 sys.exit(1)
print("OK",root.name)
