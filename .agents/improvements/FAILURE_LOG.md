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
## [AIL-20260713-171539-failure] Tab focus hooks do not run during a full Herdr session restore

**Logged**: 2026-07-13T17:15:39+08:00
**Kind**: failure
**Priority**: high
**Status**: pending
**Area**: session restore
**Related Files**: herdr-plugin.toml, internal/herdr/restore.go
**Tags**: herdr, persistence

### Trigger
User reported file tabs and trees return as terminals after closing and relaunching Herdr

### Mistake Or Rejected Approach
Assumed tab.focused would fire for the already-focused restored tab during server startup

### Correction
A named-session stop/restart proved the restored panes were zsh and plugin.log.list was empty; tab.focused only rehydrated them after an explicit tab focus

### Evidence
Herdr 0.7.3 session.json preserves launch_argv, but regular restore starts fresh shells; after restart w2:p3 and w2:p4 both reported zsh and zero plugin logs, then explicit tab focus restored both file-viewer processes

### Future Avoid Rule
Never claim automatic Herdr restart restoration from focus-hook unit tests; full-stop and restart a named session and verify process-info plus plugin logs before acceptance

---
## [AIL-20260713-171539-test-trip] Nested root assertions must match complete CLI arguments

**Logged**: 2026-07-13T17:15:39+08:00
**Kind**: test-trip
**Priority**: medium
**Status**: resolved
**Area**: testing
**Related Files**: internal/herdr/restore_test.go
**Tags**: paths, false-positive

### Trigger
A root precedence regression test initially passed while HERDR_TREE_ROOT still pointed at scripts/

### Mistake Or Rejected Approach
Used strings.Contains with the project root, which is a prefix of the nested path

### Correction
Asserted the complete --env HERDR_TREE_ROOT=<root> --direction argument boundary; the test then failed for the intended bug and passed after root precedence was fixed

### Evidence
TestRestoreFocusedTabPrefersSavedRootTreeOverNestedEventContext changed from a false green to RED showing HERDR_TREE_ROOT=<root>/scripts

### Future Avoid Rule
When testing path scoping, assert full argument boundaries or decoded argv values, never a bare path prefix

---
## [AIL-20260713-174916-correction] Default tree captured launch root and ignored source shell cd

**Logged**: 2026-07-13T17:49:16+08:00
**Kind**: correction
**Priority**: medium
**Status**: pending
**Area**: unknown


### Mistake Or Rejected Approach
workspace.created attached a tree with one-time HERDR_TREE_ROOT only; r refreshed contents without re-reading CLI pane cwd

### Correction
Pass source CLI pane ID only to default trees; poll Herdr foreground_cwd and re-root; keep file-tab trees pinned


### Future Avoid Rule
For default Herdr trees, bind to an explicit source CLI pane and verify live cd; never infer follow mode from missing file path or use a file pane as cwd source.

---
