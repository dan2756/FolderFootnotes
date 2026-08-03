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
			"(( ZSH_SUBSHELL )) && return",
			`command dirnote show --hook --from "${_dirnote_prev-}"`,
			"typeset -g _dirnote_prev=$PWD",
		},
		"bash": {
			`eval "$(dirnote init bash)"`,
			`if [[ "${_dirnote_prev-}" != "$PWD" ]]; then`,
			`PROMPT_COMMAND="_dirnote_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}"`,
			`command dirnote show --hook --from "${_dirnote_prev-}"`,
			"return $status",
		},
		"fish": {
			"dirnote init fish | source",
			"function __dirnote_hook --on-variable PWD",
			"status is-command-substitution; and return",
			`command dirnote show --hook --from "$__dirnote_prev"`,
			"set -g __dirnote_prev $PWD",
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
		// zsh and fish must also run the hook once at startup, not only on
		// cd. (bash gets that for free: the first prompt sees a new $PWD.)
		lines := strings.Split(strings.TrimSpace(script), "\n")
		if last := lines[len(lines)-1]; shell != "bash" && !strings.HasSuffix(last, "_dirnote_hook") {
			t.Errorf("%s script does not run the hook at startup", shell)
		}
	}
}

func TestScriptFlags(t *testing.T) {
	for _, shell := range Shells {
		script, err := Script(shell, "--inherit", "--plain")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(script, "command dirnote show --hook --inherit --plain --from ") {
			t.Errorf("%s script does not pass the flags on:\n%s", shell, script)
		}
	}
}

func TestScriptUnknownShell(t *testing.T) {
	if _, err := Script("tcsh"); err == nil {
		t.Fatal("expected an error for tcsh")
	}
}

// TestBashHook runs the bash hook against a stub dirnote: it must fire only
// when $PWD changes, pass the previous directory, keep an existing
// PROMPT_COMMAND and leave $? alone.
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
	stub := "#!/bin/sh\necho \"called $*\"\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "dirnote"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	start, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	prog := `PROMPT_COMMAND='echo prev'
eval "$HOOK"
eval "$HOOK"
echo "PC=$PROMPT_COMMAND"
_dirnote_hook
_dirnote_hook
cd /
(exit 7)
_dirnote_hook
echo "status=$?"
`
	cmd := exec.Command(bash, "--norc", "--noprofile", "-c", prog)
	cmd.Dir = start
	cmd.Env = append(os.Environ(), "PWD="+start, "HOOK="+script, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	want := "PC=_dirnote_hook;echo prev\n" +
		"called show --hook --from \n" +
		"called show --hook --from " + start + "\n" +
		"status=7\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}
