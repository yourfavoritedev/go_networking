package sandbox

import (
	"context"
	"io"
	"net"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestListener(t *testing.T) {
	// Establish a listener on port 0
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	t.Logf("bound to %q", listener.Addr())

	for {
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}

		go func(c net.Conn) {
			t.Logf("Connected: %q\n", c.LocalAddr())
			defer c.Close()
		}(conn)
	}
}

func TestDial(t *testing.T) {
	// Create a listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// done channel
	done := make(chan struct{})
	// spin up listeners in go-routine while we establish a connection to the client
	go func() {
		defer func() { done <- struct{}{} }()

		for {
			conn, err := listener.Accept()
			if err != nil {
				t.Log(err)
				return
			}
			t.Logf("Connected: %q\n", conn.LocalAddr())

			go func(c net.Conn) {
				defer func() {
					c.Close()
					done <- struct{}{}
				}()

				// buffer to read up to 1024 bytes at a time from the socket
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						if err != io.EOF {
							t.Error(err)
						}
						return
					}

					t.Logf("received: %q", buf[:n])
				}
			}(conn)
		}
	}()

	// connect to the listener on client-side
	t.Logf("Connecting to listener: %q...\n", listener.Addr().String())
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// terminate connecton on client-side
	conn.Close()
	<-done
	// terminate listener on client-side
	listener.Close()
	<-done
}

// DialTimeout establishes a connection to a given network. If the
// connection cannot be established within the explicit time-out duration,
// then the dialer will timeout.
func DialTimeout(
	network, address string,
	timeout time.Duration,
) (net.Conn, error) {
	// override net.Dialer with a custom control to return a DNS time-out error
	d := net.Dialer{
		Control: func(_, addr string, _ syscall.RawConn) error {
			return &net.DNSError{
				Err:         "connection timed out",
				Name:        addr,
				Server:      "127.0.0.1",
				IsTimeout:   true,
				IsTemporary: true,
			}
		},
		Timeout: timeout,
	}
	return d.Dial(network, address)
}

func TestDialTimeout(t *testing.T) {
	// attempt to dial to aa non-routable IP address, we can assume this will timeout
	c, err := DialTimeout("tcp", "10.0.0.1:http", 5*time.Second)
	// Verify that an error occurred
	if err == nil {
		c.Close()
		t.Fatal("connection did not time out")
	}
	// assert err is a net.Error
	nErr, ok := err.(net.Error)
	if !ok {
		t.Fatal(err)
	}

	// Verify that the error is a timeout
	if !nErr.Timeout() {
		t.Fatal("error is not a timeout")
	}
}

func TestDialContext(t *testing.T) {
	// set a deadline of 5 seconds into the future
	dl := time.Now().Add(5 * time.Second)
	// create context with the expected deadline for when it will automatically cancel
	ctx, cancel := context.WithDeadline(context.Background(), dl)
	// ensure we call cancel at the end of execution to ensure the context is garbage collected
	defer cancel()

	var d net.Dialer
	d.Control = func(_, _ string, _ syscall.RawConn) error {
		// Sleep long enough to reach the context's deadline
		time.Sleep(5*time.Second + time.Millisecond)
		return nil
	}
	conn, err := d.DialContext(ctx, "tcp", "10.0.0.0:80")
	if err == nil {
		conn.Close()
		t.Fatal("connection did not time out")
	}

	nErr, ok := err.(net.Error)
	if !ok {
		t.Error(err)
	} else {
		if !nErr.Timeout() {
			t.Errorf("error is not a timeout: %v", err)
		}
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("expected deadline exceeded, actual: %v", ctx.Err())
	}
}

func TestDialContextCancel(t *testing.T) {
	// create a context with a cancel function
	ctx, cancel := context.WithCancel(context.Background())
	sync := make(chan struct{})

	// connect to context and perform handshake with remote node
	go func() {
		// write to sync (done channel) before exiting.
		// this helps ensure successes and errors are handled cleanly.
		defer func() { sync <- struct{}{} }()

		var d net.Dialer
		d.Control = func(_, _ string, _ syscall.RawConn) error {
			time.Sleep(time.Second)
			return nil
		}
		conn, err := d.DialContext(ctx, "tcp", "10.0.0.1:80")
		if err != nil {
			t.Log(err)
			return
		}

		conn.Close()
		t.Error("connection did not time out")
	}()

	// abruptly cancel connection attempt
	cancel()
	// read from sync (done channel) to ensure go-routine wrote to it before exiting
	<-sync

	// verify cancel error occurred as expected
	if ctx.Err() != context.Canceled {
		t.Errorf("expected canceled context, actual: %q", ctx.Err())
	}
}

func TestDialContextCancelFanOut(t *testing.T) {
	dl := time.Now().Add(10 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), dl)

	// setup listener
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	// spin-up listener in go-routine while the client establishes a connection
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			conn.Close()
		}
	}()

	dial := func(
		ctx context.Context,
		address string,
		response chan int,
		id int,
		wg *sync.WaitGroup,
	) {
		// decrement wait group counter upon successfully writing to response channel or
		// through the context being cancelled (via the deadline or through cancel function)
		defer wg.Done()

		var d net.Dialer
		// make connection
		conn, err := d.DialContext(ctx, "tcp", address)
		if err != nil {
			return
		}
		conn.Close()

		// blocks execution until one of these cases are observed
		select {
		case <-ctx.Done():
		case response <- id:
		}
	}

	res := make(chan int)
	var wg sync.WaitGroup

	// setup multiple dialers to the listener
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go dial(ctx, listener.Addr().String(), res, i+1, &wg)
	}

	// blocks until a response can be read from the channel
	response := <-res
	// cancel will cancel the context and write to its Done channel
	cancel()
	// block until all dial goroutines terminate after the context is cancelled
	wg.Wait()
	close(res)

	if ctx.Err() != context.Canceled {
		t.Errorf("expected canceled context, actual: %s", ctx.Err())
	}

	t.Logf("dialer %d retrieved the resource", response)
}

func TestDeadline(t *testing.T) {
	sync := make(chan struct{})

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Log(err)
			return
		}
		defer func() {
			conn.Close()
			// close sync channel to ensure read from sync is no longer blocked
			close(sync)
		}()

		// set client's read deadline
		err = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			t.Error(err)
			return
		}

		buf := make([]byte, 1)
		// Read blocks until remote node sends data or deadline is exceeded
		_, err = conn.Read(buf)
		// can expect Read to return a timeout error since deadline was exceeded
		nErr, ok := err.(net.Error)
		// verify that err is a timeout
		if !ok || !nErr.Timeout() {
			t.Errorf("expected timeout error, actual: %v", err)
		}

		// write to sync channel after deadline has exceeded
		sync <- struct{}{}

		// push deadline forward, restoring its deadline functionality
		err = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			t.Error(err)
			return
		}

		// Read should now succeed now that a buffer has been written
		_, err = conn.Read(buf)
		if err != nil {
			t.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// block until we can read from channel or is closed
	<-sync
	// write data to connecton
	_, err = conn.Write([]byte("1"))
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err != io.EOF {
		t.Errorf("expected server termination, actual: %v", err)
	}
}

func TestTemporaryErrors(t *testing.T) {
	var (
		err error
		n   int
		i   = 7 // maximum number of retries
	)

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Error(err)
			return
		}

		defer conn.Close()

		for ; i > 0; i-- {
			// attempt to write to connection
			n, err = conn.Write([]byte("hello world"))
			if err != nil {
				// verify if err is temporary
				if nErr, ok := err.(net.Error); ok && nErr.Temporary() {
					t.Logf("temporary error: %v", nErr)
					// wait five seconds before retrying to write
					time.Sleep(5 * time.Second)
					continue
				}
				// exit because this was not a temporary error
				return
			}
			// break out of loop if write was successful
			break
		}

		if i == 0 {
			t.Log("temporary write failure threshold execeeded")
			return
		}

		t.Logf("wrote %d bytes to %s\n", n, conn.RemoteAddr())
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	defer conn.Close()

	buf := make([]byte, 1<<19)
	_, err = conn.Read(buf)
	if err != nil {
		if err != io.EOF {
			t.Error(err)
		}
	}
}
