# Avoid Rules

Short imperative rules for future agents.

- **AIL-20260713-072751-correction**: Whenever a pane's cwd is repurposed as hidden identity metadata, never let child or sibling UI panes infer their content root from that cwd; pass the user-visible project root explicitly and verify it with a nested-file live test.
- **AIL-20260713-073219-failure**: For this repository, inspect the exact current lines before every apply_patch after prior edits; do not reuse remembered multi-file context across TDD iterations.
- **AIL-20260713-171539-failure**: Never claim automatic Herdr restart restoration from focus-hook unit tests; full-stop and restart a named session and verify process-info plus plugin logs before acceptance
- **AIL-20260713-171539-test-trip**: When testing path scoping, assert full argument boundaries or decoded argv values, never a bare path prefix
