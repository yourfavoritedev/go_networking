package monitor

import (
	"io"
	"log"
	"net"
	"os"
)

// Monitor embed a log.Logger meant for logging network traffic
type Monitor struct {
	*log.Logger
}

// Write implements the io.Writer interface
func (m *Monitor) Write(p []byte) (int, error) {
	err := m.Output(2, string(p))
	if err != nil {
		log.Println(err)
	}

	return len(p), nil
}

func ExampleMonitor() {
	monitor := &Monitor{
		Logger: log.New(os.Stdout, "monitor: ", 0),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		monitor.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		serverConn, err := listener.Accept()
		if err != nil {
			return
		}
		defer serverConn.Close()

		b := make([]byte, 1024)
		// create reader that reads from network connection and
		// writes all input to the monitor
		r := io.TeeReader(serverConn, monitor)

		// read message from connection - triggers log
		n, err := r.Read(b)
		if err != nil && err != io.EOF {
			monitor.Println(err)
			return
		}

		// setup multi-writer for the network connection and the monitor
		w := io.MultiWriter(serverConn, monitor)

		// write to all writers
		_, err = w.Write(b[:n]) // echo the message
		if err != nil && err != io.EOF {
			monitor.Println(err)
			return
		}
	}()

	// setup client-connection
	clientConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		monitor.Fatal(err)
	}

	_, err = clientConn.Write([]byte("Test\n"))
	if err != nil {
		monitor.Fatal(err)
	}

	_ = clientConn.Close()
	<-done
}
