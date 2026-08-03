// Package shell produces the snippets that make a shell run
// "dirnote show --hook" whenever the working directory changes.
package shell

import (
	"fmt"
	"strings"
)

// Shells lists the supported shells.
var Shells = []string{"bash", "fish", "zsh"}

// Script returns the hook for the named shell, meant to be eval'd from the
// shell's startup file. flags are added to the "dirnote show --hook" call;
// they are spliced in verbatim, so callers must pass known flags only.
//
// Every hook remembers the directory it last ran in and passes it as
// --from, which is what lets --inherit tell entering a project apart from
// moving around inside it.
func Script(shell string, flags ...string) (string, error) {
	var tmpl string
	switch shell {
	case "zsh":
		tmpl = zsh
	case "bash":
		tmpl = bash
	case "fish":
		tmpl = fish
	default:
		return "", fmt.Errorf("unsupported shell %q (want %s)", shell, strings.Join(Shells, ", "))
	}
	cmd := strings.Join(append([]string{"command dirnote show --hook"}, flags...), " ")
	return strings.ReplaceAll(tmpl, "@CMD@", cmd), nil
}

// The zsh hook skips subshells so that $(cd dir && ...) stays quiet.
const zsh = `# dirnote hook for zsh. Add to ~/.zshrc:
#   eval "$(dirnote init zsh)"
_dirnote_hook() {
  (( ZSH_SUBSHELL )) && return
  @CMD@ --from "${_dirnote_prev-}"
  typeset -g _dirnote_prev=$PWD
}
autoload -Uz add-zsh-hook
add-zsh-hook chpwd _dirnote_hook
_dirnote_hook
`

// bash has no chpwd hook, so the prompt command watches $PWD instead. Any
// existing PROMPT_COMMAND is kept, and $? is passed through untouched for
// prompts that show the last exit status.
const bash = `# dirnote hook for bash. Add to ~/.bashrc:
#   eval "$(dirnote init bash)"
_dirnote_hook() {
  local status=$?
  if [[ "${_dirnote_prev-}" != "$PWD" ]]; then
    @CMD@ --from "${_dirnote_prev-}"
    _dirnote_prev=$PWD
  fi
  return $status
}
if [[ "${PROMPT_COMMAND[*]-}" != *_dirnote_hook* ]]; then
  PROMPT_COMMAND="_dirnote_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
`

const fish = `# dirnote hook for fish. Add to ~/.config/fish/config.fish:
#   dirnote init fish | source
function __dirnote_hook --on-variable PWD
    status is-command-substitution; and return
    @CMD@ --from "$__dirnote_prev"
    set -g __dirnote_prev $PWD
end
__dirnote_hook
`
