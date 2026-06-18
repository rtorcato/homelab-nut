<!--
Thanks for the PR! Quick checklist:
  - Branch name follows `feat/`, `fix/`, `docs/`, `chore/`, `ci/`, `refactor/`, `test/`
    (with the issue number when one exists: `feat/20-nut-client-role`).
  - PR title is a Conventional Commit (`feat(roles): add nut-client role`) —
    it becomes the squash commit on main.
  - `make lint && make test && make embed-sync` is clean locally.
  - If you touched scripts/*.sh, you ran `make embed-sync` so the embedded
    copy under internal/roles/embedded/ doesn't drift (CI enforces this).
  - Every new TODO/FIXME in code references an issue: `TODO(#42): …`.

Once you're ready, queue auto-merge:
  gh pr merge --auto --squash --delete-branch
-->

## What

<!-- One or two sentences. Which package(s) or area(s) are affected? -->

## Why

<!-- The user-facing motivation, bug, or use case. Link the issue: Closes #N -->

## How

<!--
  Implementation notes a reviewer would want to know:
  - New abstractions, interfaces, files
  - Anything non-obvious in the diff
  - Migration steps for users (if any)
  - Skip this section for trivial PRs.
-->

<!--
Closes #
-->
