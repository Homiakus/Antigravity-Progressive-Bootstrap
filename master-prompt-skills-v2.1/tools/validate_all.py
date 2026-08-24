#!/usr/bin/env python3
from pathlib import Path
import subprocess,sys
root=Path(__file__).resolve().parents[1]
fails=0
for d in sorted((root/"skills").iterdir()):
 if d.is_dir():
  r=subprocess.run([sys.executable,str(d/"scripts"/"validate_skill.py")])
  fails += (r.returncode != 0)
sys.exit(1 if fails else 0)
