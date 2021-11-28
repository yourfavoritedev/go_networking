package proxy

import (
	"io"
)

// proxy can be used if the following conditions are met:
// 1. from (io.Reader) implements the io.Writer interface
// 2. to (io.Writer) implements the io.Reader interface
func proxy(from io.Reader, to io.Writer) error {
	// assert that from implements io.Writer
	fromWriter, fromIsWriter := from.(io.Writer)
	// assert that to implements io.Reader
	toReader, toIsReader := to.(io.Reader)

	// if both interfaces are implemented then we can use io.Copy to copy data
	// into each other
	if toIsReader && fromIsWriter {
		// Send replies from sending server (to) to destination server (from).
		go func() {
			_, _ = io.Copy(fromWriter, toReader)
		}()
	}

	// proxies the replies back from the destination server to the sending server
	_, err := io.Copy(to, from)

	return err
}
