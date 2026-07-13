# Screenshots

The current installed plugin UX is represented by:

- `right-side-tree.txt`
- `file-tab-with-tree.txt`

The older `browse.txt`, `staging.txt`, `git-branches.txt`, `help.txt`, and
`browse.png` files are retained as legacy references for the upstream-style
full viewer that still exists when the binary is run without `--tree-only`.

For colour screenshots, run the plugin against any project and capture your
terminal, then drop the PNGs here and reference them from the top-level README:

```sh
go build -o bin/file-viewer ./cmd/file-viewer
./bin/file-viewer /path/to/some/project
```
