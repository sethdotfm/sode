#!/usr/bin/env bash
# PostToolUse (Write|Edit): format the edited file, then lint it.
# Go: gofmt -w, then go vet on the package. web/: Prettier, then node --check for JS.
# Exit 2 sends the tool's stderr back to Claude; anything else is silent.
set -u

f=$(jq -r '.tool_response.filePath // .tool_input.file_path // empty')
[ -n "$f" ] && [ -f "$f" ] || exit 0

# The file's own checkout, not CLAUDE_PROJECT_DIR: that names the session's checkout, which is wrong for a file in another worktree.
root=$(git -C "$(dirname "$f")" rev-parse --show-toplevel 2>/dev/null) || root=${CLAUDE_PROJECT_DIR:-}
[ -n "$root" ] || exit 0
cd "$root" || exit 0
rel=${f#"$root"/}

case "$rel" in
*.go)
  gofmt -w "$f" || exit 2
  pkg=$(dirname "$rel")
  out=$(go vet "./$pkg" 2>&1) || {
    echo "go vet ./$pkg failed:" >&2
    echo "$out" >&2
    exit 2
  }
  ;;
web/vendor/*) ;;
web/*.js | web/test/*.mjs | web/*.css)
  out=$(npx --yes prettier@3.3.3 --write --ignore-unknown "$f" 2>&1) || {
    echo "prettier failed on $rel:" >&2
    echo "$out" >&2
    exit 2
  }
  case "$rel" in
  *.js | *.mjs)
    out=$(node --check "$f" 2>&1) || {
      echo "node --check failed on $rel:" >&2
      echo "$out" >&2
      exit 2
    }
    out=$(npx --yes eslint@10.14.0 "$f" 2>&1) || {
      echo "eslint failed on $rel:" >&2
      echo "$out" >&2
      exit 2
    }
    ;;
  esac
  ;;
esac
exit 0
