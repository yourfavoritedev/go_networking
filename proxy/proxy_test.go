package proxy

import (
	"io"
	"net"
	"sync"
	"testing"
)

func TestProxy(t *testing.T) {
	var wg sync.WaitGroup

	// server listens for a "ping" message and responds with a
	// "pong" message. All other messages are echoed back to the client
	serverConn, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			conn, err := serverConn.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()

				for {
					buf := make([]byte, 1024)
					n, err := c.Read(buf)
					if err != nil {
						if err != io.EOF {
							t.Error(err)
						}

						return
					}

					switch msg := string(buf[:n]); msg {
					case "ping":
						_, err = c.Write([]byte("pong"))
					default:
						_, err = c.Write(buf[:n])
					}
				}
			}(conn)
		}
	}()

	// proxyServer proxies messages from client connections to the destinationServer.
	// Replies from the destinationServer are proxied back to the clients.
	proxyServer, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			conn, err := proxyServer.Accept()
			if err != nil {
				return
			}

			go func(from net.Conn) {
				defer from.Close()

				to, err := net.Dial("tcp", serverConn.Addr().String())
				if err != nil {
					t.Error(err)
					return
				}

				defer to.Close()

				err = proxy(from, to)
				if err != nil && err != io.EOF {
					t.Error(err)
				}
			}(conn)
		}
	}()

	clientConn, err := net.Dial("tcp", proxyServer.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	msgs := []struct {
		Message string
		Reply   string
	}{
		{"ping", "pong"},
		{"pong", "pong"},
		{"echo", "echo"},
		{"ping", "pong"},
	}

	for i, m := range msgs {
		_, err = clientConn.Write([]byte(m.Message))
		if err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, 1024)

		n, err := clientConn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}

		actual := string(buf[:n])
		t.Logf("%q -> proxy -> %q", m.Message, actual)

		if actual != m.Reply {
			t.Errorf("%d: expected reply: %q, actual: %q",
				i, m.Reply, actual)
		}
	}

	_ = clientConn.Close()
	_ = proxyServer.Close()
	_ = serverConn.Close()

	wg.Wait()
}
