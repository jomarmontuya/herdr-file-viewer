# Avoid Rules

Short imperative rules for future agents.

- **AIL-20260713-072751-correction**: Whenever a pane's cwd is repurposed as hidden identity metadata, never let child or sibling UI panes infer their content root from that cwd; pass the user-visible project root explicitly and verify it with a nested-file live test.
- **AIL-20260713-073219-failure**: For this repository, inspect the exact current lines before every apply_patch after prior edits; do not reuse remembered multi-file context across TDD iterations.
