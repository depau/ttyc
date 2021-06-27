package shenanigans

import (
	"golang.org/x/sys/windows"
	"os"
)

func WindowsClearConsole() (err error) {
	stdout := windows.Handle(os.Stdout.Fd())
	var outMode uint32
	if err = windows.GetConsoleMode(stdout, &outMode); err != nil {
		return
	}
	if err = windows.SetConsoleMode(stdout, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		_ = windows.SetConsoleMode(stdout, outMode)
		return
	}
	_, err = os.Stdout.Write([]byte("\u001b[2J"))
	// ignore error but still return it
	err = windows.SetConsoleMode(stdout, outMode)
	return
}
