#!/bin/sh
# Make the demo GIF for the README.
# Usage: sh assets/demo.sh
# Needs: asciinema, agg and DejaVu (brew install asciinema agg font-dejavu),
# expect and python3.
set -e
root=$(cd "$(dirname "$0")/.." && pwd)
demo=$(cd "${TMPDIR:-/tmp}" && pwd -P)/observatory-demo
rm -rf "$demo"
mkdir -p "$demo/bin"
go build -o "$demo/bin/observatory" "$root/cmd/observatory"
export PATH="$demo/bin:$PATH"

# A small repository with two rules, and a commit that breaks one of them.
home=$HOME
export HOME="$demo"
git init -q -b main "$demo/myrepo"
cd "$demo/myrepo"
observatory init >/dev/null
observatory add "Use httpx for HTTP calls" >/dev/null
printf '# Use httpx for HTTP calls\n\nEvery HTTP call goes through httpx; do not import requests or urllib.\n' > .observatory/OBS-001/rule.md
observatory add "Always use ASD-STE100 for documentation" >/dev/null
printf '# Always use ASD-STE100 for documentation\n\nWrite the documentation in Simplified Technical English.\n' > .observatory/OBS-002/rule.md
printf 'import requests\n\ndef fetch(url):\n    return requests.get(url).json()\n' > client.py
git add -A
git -c user.name=demo -c user.email=demo@example.com commit -q -m "add the client"

# A local replacement for the API, so that the demo needs no key and no network.
python3 "$root/assets/demo_api.py" &
trap 'kill $!' EXIT
export OBSERVATORY_BASE_URL=http://127.0.0.1:8089 ANTHROPIC_API_KEY=demo

export TERM=xterm-256color COLORTERM=truecolor
asciinema rec --headless --overwrite --window-size 128x14 \
  -c "expect $root/assets/demo.exp" "$demo/demo.cast"
# The default font of agg has no glyph for the spinner. The fontdue renderer
# takes the glyph from the next font in the list.
HOME=$home agg --renderer fontdue --font-family "DejaVu Sans Mono,DejaVu Sans" --font-size 14 --theme monokai \
  "$demo/demo.cast" "$root/assets/demo.gif"
