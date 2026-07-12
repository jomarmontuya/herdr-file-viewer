# Failure Log

## [AIL-20260713-072751-correction] File-tab trees inherited the selected file's parent directory

**Logged**: 2026-07-13T07:27:51+08:00
**Kind**: correction
**Priority**: medium
**Status**: pending
**Area**: unknown


### Mistake Or Rejected Approach
The v0.5.0 file-tab flow launched the file pane with --cwd set to filepath.Dir(path) for exact tab ownership, then opened the attached viewer without an explicit project root. Herdr propagated the file pane cwd into the tree context, so nested files scoped the tree to their parent folder.

### Correction
Carry the original tree model root through OpenFileTab and pass it explicitly when opening the attached viewer pane, while retaining the file pane's parent cwd solely for exact ownership validation.


### Future Avoid Rule
Whenever a pane's cwd is repurposed as hidden identity metadata, never let child or sibling UI panes infer their content root from that cwd; pass the user-visible project root explicitly and verify it with a nested-file live test.

---
## [AIL-20260713-073219-failure] Repeated apply_patch context mismatches during Herdr fixes

**Logged**: 2026-07-13T07:32:19+08:00
**Kind**: failure
**Priority**: medium
**Status**: pending
**Area**: unknown


### Mistake Or Rejected Approach
Several multi-file and test-only patches were authored against remembered whitespace or stale surrounding lines, causing apply_patch verification failures even though no files were changed.

### Correction
Read the exact target block immediately before patching and apply small file-scoped hunks with only the minimum stable context.


### Future Avoid Rule
For this repository, inspect the exact current lines before every apply_patch after prior edits; do not reuse remembered multi-file context across TDD iterations.

---
