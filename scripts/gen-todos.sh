#!/usr/bin/env bash
# Regenerate TODOS.md from GitHub Issues.
#
# Lists every issue with a `phase-*` label (excluding the Epic issues
# themselves, which become section headings). Bugs and other one-off
# issues are intentionally not included — track those in GitHub directly.
#
# Requires: gh CLI authenticated against the homelab-nut repo, jq.

set -euo pipefail

REPO="${REPO:-rtorcato/homelab-nut}"
OUT="${OUT:-TODOS.md}"
DATE="$(date -u +%Y-%m-%d)"
ISSUE_URL_BASE="https://github.com/${REPO}/issues"

if ! command -v gh >/dev/null; then
    echo "gh CLI is required" >&2
    exit 1
fi
if ! command -v jq >/dev/null; then
    echo "jq is required" >&2
    exit 1
fi

# Pull every issue once. The labels we care about are inferred per-phase below.
ALL_JSON="$(gh issue list \
    --repo "$REPO" \
    --state all \
    --limit 200 \
    --json number,title,state,labels,milestone)"

issues_for_phase() {
    local phase="$1"
    # All issues with this phase label, sorted by number, excluding Epic issues.
    jq -r --arg ph "$phase" '
        map(select(any(.labels[]; .name == $ph) and (.title | startswith("Epic — ") | not)))
        | sort_by(.number)
        | .[]
        | "\(.state)\t\(.number)\t\(.title)\t\(.milestone.title // "")"
    ' <<<"$ALL_JSON"
}

epic_for_phase() {
    local phase="$1"
    # The Epic issue carrying this phase label.
    jq -r --arg ph "$phase" '
        map(select(any(.labels[]; .name == $ph) and (.title | startswith("Epic — "))))
        | first.number
    ' <<<"$ALL_JSON"
}

write_phase() {
    local phase="$1"
    local heading="$2"
    local milestone_hint="$3"

    local epic
    epic="$(epic_for_phase "$phase")"

    local lines
    lines="$(issues_for_phase "$phase" || true)"

    local closed=0 open=0
    if [[ -n "$lines" ]]; then
        closed=$(awk -F'\t' '$1 == "CLOSED"' <<<"$lines" | wc -l | tr -d ' ')
        open=$(awk -F'\t' '$1 == "OPEN"' <<<"$lines" | wc -l | tr -d ' ')
    fi
    local total=$((closed + open))

    {
        printf '\n## %s' "$heading"
        if [[ "$epic" != "null" ]]; then
            printf ' ([epic #%s](%s/%s))' "$epic" "$ISSUE_URL_BASE" "$epic"
        fi
        if [[ -n "$milestone_hint" ]]; then
            printf ' · %s' "$milestone_hint"
        fi
        printf '\n\n'

        if [[ "$total" -eq 0 ]]; then
            printf '_No children filed yet._\n'
            return
        fi

        printf 'Progress: %d / %d closed\n\n' "$closed" "$total"

        while IFS=$'\t' read -r state number title _; do
            [[ -z "$number" ]] && continue
            local box='[ ]'
            [[ "$state" == "CLOSED" ]] && box='[x]'
            # Escape pipes in titles so they don't break tables (not used today,
            # but cheap insurance for any future markdown rendering).
            local safe_title="${title//|/\\|}"
            printf -- '- %s [#%s — %s](%s/%s)\n' \
                "$box" "$number" "$safe_title" "$ISSUE_URL_BASE" "$number"
        done <<<"$lines"
    }
}

{
    cat <<'EOF'
# TODOs

Roadmap work tracked as GitHub Issues, grouped by phase. **Generated** — run `make todos` or `scripts/gen-todos.sh` to refresh.

> Bugs and one-off issues are tracked on GitHub directly and not listed here. This file is the index of *planned* work (issues with a `phase-*` label).

EOF
    printf -- '_Last updated: %s_\n' "$DATE"

    write_phase phase-0 'Phase 0 — Foundation'           ''
    write_phase phase-1 'Phase 1 — Skeleton'             'v0.1 Alpha'
    write_phase phase-2 'Phase 2 — SSH + first apply'    'v0.1 Alpha'
    write_phase phase-3 'Phase 3 — All roles + status'   'v0.2 Beta'
    write_phase phase-4 'Phase 4 — Polish + distribution' 'v1.0 GA'
    write_phase phase-5 'Phase 5 — Promotion'            'v1.0 GA'
    write_phase phase-6 'Phase 6 — Native port'          ''

    cat <<'EOF'

---

[Browse all issues](https://github.com/rtorcato/homelab-nut/issues) · [Bugs](https://github.com/rtorcato/homelab-nut/issues?q=is%3Aissue+is%3Aopen+label%3Abug) · [Good first issues](https://github.com/rtorcato/homelab-nut/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
EOF
} >"$OUT"

echo "wrote $OUT"
