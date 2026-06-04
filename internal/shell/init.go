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
// shell's startup file.
func Script(shell string) (string, error) {
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
	return strings.ReplaceAll(tmpl, "@CMD@", "command dirnote show --hook"), nil
}

const zsh = `# dirnote hook for zsh. Add to ~/.zshrc:
#   eval "$(dirnote init zsh)"
_dirnote_hook() {
  @CMD@
}
autoload -Uz add-zsh-hook
add-zsh-hook chpwd _dirnote_hook
_dirnote_hook
`

// bash has no chpwd hook, so the prompt command watches $PWD instead. Any
// existing PROMPT_COMMAND is kept.
const bash = `# dirnote hook for bash. Add to ~/.bashrc:
#   eval "$(dirnote init bash)"
_dirnote_hook() {
  if [[ "${_dirnote_pwd-}" != "$PWD" ]]; then
    _dirnote_pwd=$PWD
    @CMD@
  fi
}
if [[ "${PROMPT_COMMAND[*]-}" != *_dirnote_hook* ]]; then
  PROMPT_COMMAND="_dirnote_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
`

const fish = `# dirnote hook for fish. Add to ~/.config/fish/config.fish:
#   dirnote init fish | source
function __dirnote_hook --on-variable PWD
    @CMD@
end
__dirnote_hook
`
