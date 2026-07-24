#!/usr/bin/env bash
# Promote the [Unreleased] section of CHANGELOG.md to a versioned release
# section, refresh the Unreleased placeholder, and rebuild the compare-link
# footer. Run locally before tagging a release — see RELEASING.md.
#
# Usage: scripts/release-changelog.sh <version> [date]
#   version : X.Y.Z (no leading v)
#   date    : YYYY-MM-DD (default: today, UTC)
#
# The workflow is Keep a Changelog: contributors accumulate rich entries under
# [Unreleased] per PR; this script moves them under the new version. If
# [Unreleased] is empty, it falls back to generating entries from
# conventional-commit subjects since the previous tag (feat→Added, fix→Fixed,
# perf/refactor→Changed, feat!/fix!/BREAKING→Breaking; docs/chore/ci/test/
# build/style excluded — the same filter goreleaser uses for release notes).
set -euo pipefail

REPO="https://github.com/kubespaces-io/kubespaces"
ROOT="$(git rev-parse --show-toplevel)"
CHANGELOG="$ROOT/CHANGELOG.md"

VERSION="${1:?usage: release-changelog.sh <version> [date]}"
DATE="${2:-$(date -u +%Y-%m-%d)}"

echo "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' \
  || { echo "error: version must be X.Y.Z (got '$VERSION')" >&2; exit 1; }
grep -q '^## \[Unreleased\]' "$CHANGELOG" \
  || { echo "error: no '## [Unreleased]' section in $CHANGELOG" >&2; exit 1; }
grep -q "^## \[$VERSION\]" "$CHANGELOG" \
  && { echo "error: version $VERSION already present in changelog" >&2; exit 1; }

# Previous version = the first (newest) version header currently in the file.
PREV="$(grep -oE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' "$CHANGELOG" \
  | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || true)"

# Current [Unreleased] body: everything between its header and the next '## ',
# with leading/trailing blank lines trimmed.
BODY="$(awk '
  /^## \[Unreleased\]/ {f=1; next}
  /^## /               {f=0}
  f                    {print}
' "$CHANGELOG" | awk '
  {lines[NR]=$0}
  END{ s=1; while (s<=NR && lines[s]=="") s++;
       e=NR; while (e>=1 && lines[e]=="") e--;
       for (i=s; i<=e; i++) print lines[i] }
')"

# Fallback: synthesize entries from conventional commits since vPREV.
gen_from_commits() {
  local range="HEAD"
  if [ -n "$PREV" ] && git rev-parse -q --verify "v$PREV" >/dev/null 2>&1; then
    range="v$PREV..HEAD"
  fi
  local subjects; subjects="$(git log --no-merges --pretty=%s "$range")"
  emit() { # $1=heading  $2=grep-ERE
    local picked
    picked="$(printf '%s\n' "$subjects" | grep -E "$2" \
      | sed -E 's/^[a-z]+(\([^)]*\))?!?:[[:space:]]*//' | sed 's/^/- /' || true)"
    [ -n "$picked" ] && printf '\n### %s\n%s\n' "$1" "$picked"
  }
  emit "Breaking" '^[a-z]+(\([^)]*\))?!:'
  emit "Added"    '^feat(\([^)]*\))?:'
  emit "Fixed"    '^fix(\([^)]*\))?:'
  emit "Changed"  '^(perf|refactor)(\([^)]*\))?:'
}

if [ -z "$BODY" ]; then
  BODY="$(gen_from_commits)"
  BODY="${BODY#$'\n'}"   # drop one leading newline from the first emit()
fi
[ -n "$BODY" ] || BODY="_No user-facing changes._"

# Replacement for the single '## [Unreleased]' line: keep an empty Unreleased
# placeholder, then open the new version section (its body follows below).
# Replacement block, written to a file so awk can inject it with getline
# (BSD awk rejects multi-line strings passed via -v).
replf="$(mktemp)"
cat > "$replf" <<EOF
## [Unreleased]

## [$VERSION] — $DATE

$BODY
EOF

tmp="$(mktemp)"
# Splice in the new section and drop the old footer link refs in one pass.
awk -v replfile="$replf" '
  /^## \[Unreleased\][[:space:]]*$/ {
    while ((getline line < replfile) > 0) print line
    close(replfile)
    print ""                 # exactly one blank line before the next section
    inbody=1; next           # skip the ORIGINAL Unreleased body — it moves up
  }
  inbody && /^## /  { inbody=0 }          # next section header: stop skipping
  inbody            { next }              # drop old Unreleased body lines
  /^\[[^][]+\]:[[:space:]]/ { next }      # strip footer reference links
  { print }
' "$CHANGELOG" > "$tmp"
rm -f "$replf"

# Trim trailing blank lines, then append a freshly-computed footer.
awk '
  {lines[NR]=$0}
  END{ e=NR; while (e>=1 && lines[e]=="") e--;
       for (i=1; i<=e; i++) print lines[i] }
' "$tmp" > "$tmp.trim" && mv "$tmp.trim" "$tmp"

# Footer: Unreleased + one entry per version header, newest first. Adjacent
# versions get compare links; the oldest points at its release tag.
VERS=()
while IFS= read -r v; do VERS+=("$v"); done < <(
  grep -oE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' "$tmp" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+'
)
{
  echo ""
  echo "[Unreleased]: $REPO/compare/v${VERS[0]}...HEAD"
  for ((i=0; i<${#VERS[@]}; i++)); do
    cur="${VERS[i]}"
    if (( i+1 < ${#VERS[@]} )); then
      echo "[$cur]: $REPO/compare/v${VERS[i+1]}...v$cur"
    else
      echo "[$cur]: $REPO/releases/tag/v$cur"
    fi
  done
} >> "$tmp"

mv "$tmp" "$CHANGELOG"
echo "Promoted [Unreleased] → [$VERSION] — $DATE (previous: ${PREV:-none})"
