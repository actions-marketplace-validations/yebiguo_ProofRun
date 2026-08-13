# VHS tape for the README demo GIF. Run with: vhs docs/demo.tape
# Regenerates docs/demo.gif from a from-scratch throwaway repo — fully
# self-contained, no pre-existing state required. Requires `proofrun` and
# `python`/`pytest` on PATH. See https://github.com/charmbracelet/vhs

Output docs/demo.gif

Set Shell "bash"
Set FontSize 20
Set Width 1100
Set Height 620
Set Theme "Dracula"
Set Padding 24
Set TypingSpeed 60ms

Hide
Type `rm -rf /tmp/proofrun-demo && mkdir -p /tmp/proofrun-demo/src && cd /tmp/proofrun-demo`
Enter
Type `git init -q && git config user.email demo@example.com && git config user.name demo`
Enter
Type `printf 'def add(a, b):\n    return a + b\n' > src/app.py`
Enter
Type `printf 'from src.app import add\n\ndef test_add():\n    assert add(2, 2) == 4\n' > test_app.py`
Enter
Type `printf '__pycache__/\n.pytest_cache/\n' > .gitignore`
Enter
Type `printf 'checks:\n  test:\n    command: [python, -m, pytest]\n    required: true\n' > .proofrun.yml`
Enter
Type `git add . && git commit -qm init`
Enter
Type "clear"
Enter
Show

Type "proofrun run test -- python -m pytest"
Sleep 500ms
Enter
Sleep 2500ms

Type "proofrun status"
Sleep 500ms
Enter
Sleep 2000ms

Type "echo '# a change nobody re-ran the test on' >> src/app.py"
Sleep 500ms
Enter
Sleep 1000ms

Type "proofrun status"
Sleep 500ms
Enter
Sleep 3000ms
