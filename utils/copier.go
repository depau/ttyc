package utils

import (
	"fmt"
	"io"
)

// outChan should have len == 1, so writing to it blocks, there is no buffering and the buffer isn't overwritten
func CopyReaderToChan(closeChan <-chan interface{}, fd io.Reader, outChan chan<- []byte, errChan chan<- error) {
	useBuf2 := false
	buffer1 := make([]byte, 4096)
	buffer2 := make([]byte, 4096)
	for {
		select {
		case <-closeChan:
			return
		default:
		}

		var buf []byte
		if useBuf2 {
			buf = buffer2
		} else {
			buf = buffer1
		}
		useBuf2 = !useBuf2

		//println("BLOCKING CopyReaderToChan")
		bRead, err := fd.Read(buf)
		//println("Unblocked CopyReaderToChan")

		if err != nil {
			errChan <- fmt.Errorf("tty error (usually terminal closed), shutting down: %v", err)
			return
		}
		outChan <- buf[0:bRead]
	}
}

func CopyChanToWriter(closeChan <-chan interface{}, inChan <-chan []byte, fd io.Writer, errChan chan<- error) {
	for {
		//println("SELECT CopyChanToWriter")
		select {
		case <-closeChan:
			return
		case buf := <-inChan:
			written := 0
			for written < len(buf) {
				bWritten, err := fd.Write(buf[written:])
				if err != nil {
					errChan <- err
					return
				}
				written += bWritten
			}
		}
		//println("SELECTED CopyChanToWriter")
	}
}
