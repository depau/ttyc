package ttyc

import (
	"fmt"
	"runtime"
)

// https://stackoverflow.com/a/46289376/1124621
func Trace() {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	fmt.Printf("%s:%d %s\n", frame.File, frame.Line, frame.Function)
}
