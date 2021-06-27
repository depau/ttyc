package shenanigans

import "os"

// At least on iTerm2 it looks like launching a full-screen application such as tmux inside the emulated terminal
// will result in a dirty screen on exit.
// Clean it up with the ANSI \033c escape sequence. We don't need to clear the scrollback, just the current screen.

func ClearConsole() (err error) {
	_, err = os.Stdout.Write([]byte{0x1b, 'c'})
	return
}
