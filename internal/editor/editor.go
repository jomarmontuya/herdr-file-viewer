// Package editor resolves the user's configured editors. The plugin never
// bundles one — editors are listed in a config file so any number can be
// configured, with an optional default. Resolution:
//
//	$HERDR_PLUGIN_CONFIG_DIR/editors   (one "name = command" per line; a
//	                                    leading "*" marks the default)
//	$VISUAL / $EDITOR / FILE_VIEWER_EDITOR  (fallback if no config file)
//
// The target file or project directory is appended as the final argument, so
// "code", "zed", "nvim", "code --wait" etc. all work for both files and folders.
package editor

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Editor is one configured editor.
type Editor struct {
	Name    string
	Cmd     []string
	Default bool
}

// Command builds the exec.Cmd to open target (a file or directory) in e.
func (e Editor) Command(target string) *exec.Cmd {
	args := make([]string, 0, len(e.Cmd))
	args = append(args, e.Cmd[1:]...)
	args = append(args, target)
	return exec.Command(e.Cmd[0], args...)
}

// ConfigPath returns the editors config file path, or "" if unknown.
func ConfigPath() string {
	if dir := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "editors")
	}
	return ""
}

// Load returns the configured editors: from the config file if present,
// otherwise a single editor derived from $VISUAL/$EDITOR/FILE_VIEWER_EDITOR.
func Load() []Editor {
	if p := ConfigPath(); p != "" {
		if eds := parseFile(p); len(eds) > 0 {
			return eds
		}
	}
	for _, key := range []string{"FILE_VIEWER_EDITOR", "VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			f := strings.Fields(v)
			return []Editor{{Name: f[0], Cmd: f, Default: true}}
		}
	}
	return nil
}

// Preferred returns the editor to use without prompting: the one marked default,
// or the sole configured editor. ok=false means the caller should ask.
func Preferred(eds []Editor) (Editor, bool) {
	for _, e := range eds {
		if e.Default {
			return e, true
		}
	}
	if len(eds) == 1 {
		return eds[0], true
	}
	return Editor{}, false
}

// parseFile reads "name = command" lines; a leading "*" on the name marks the
// default. Blank lines and #-comments are ignored.
func parseFile(path string) []Editor {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var eds []Editor
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, cmd, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		def := strings.HasPrefix(name, "*")
		if def {
			name = strings.TrimSpace(strings.TrimPrefix(name, "*"))
		}
		fields := strings.Fields(strings.TrimSpace(cmd))
		if name == "" || len(fields) == 0 {
			continue
		}
		eds = append(eds, Editor{Name: name, Cmd: fields, Default: def})
	}
	return eds
}
