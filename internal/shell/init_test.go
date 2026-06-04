package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptContent(t *testing.T) {
	tests := map[string][]string{
		"zsh": {
			`eval "$(dirnote init zsh)"`,
			"autoload -Uz add-zsh-hook",
			"add-zsh-hook chpwd _dirnote_hook",
			"command dirnote show --hook",
		},
		"bash": {
			`eval "$(dirnote init bash)"`,
			`!= "$PWD"`,
			`PROMPT_COMMAND="_dirnote_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}"`,
			"command dirnote show --hook",
		},
		"fish": {
			"dirnote init fish | source",
			"function __dirnote_hook --on-variable PWD",
			"command dirnote show --hook",
		},
	}
	for shell, wants := range tests {
		script, err := Script(shell)
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		for _, w := range wants {
			if !strings.Contains(script, w) {
				t.Errorf("%s script lacks %q:\n%s", shell, w, script)
			}
		}
		if strings.Contains(script, "@") {
			t.Errorf("%s script has an unexpanded placeholder:\n%s", shell, script)
		}
	}
}

func TestScriptUnknownShell(t *testing.T) {
	if _, err := Script("tcsh"); err == nil {
		t.Fatal("expected an error for tcsh")
	}
}

// TestBashHook runs the bash hook against a stub dirnote to check that it
// fires only when $PWD changes and leaves an existing PROMPT_COMMAND alone.
func TestBashHook(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	script, err := Script("bash")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	stub := "#!/bin/sh\necho \"called $*\"\n"
	if err := os.WriteFile(filepath.Join(bin, "dirnote"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	prog := `PROMPT_COMMAND='echo prev'
eval "$HOOK"
eval "$HOOK"
echo "PC=$PROMPT_COMMAND"
_dirnote_hook
_dirnote_hook
cd /
_dirnote_hook
`
	cmd := exec.Command(bash, "--norc", "--noprofile", "-c", prog)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "HOOK="+script, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	want := "PC=_dirnote_hook;echo prev\ncalled show --hook\ncalled show --hook\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}
