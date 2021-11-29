package echo

import (
	"context"
	"fmt"
	"net"
)

func echoServerUDP(ctx context.Context, addr string) (net.Addr, error) {
	// create UDP connection for the server
	server, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("binding to udp %s: %w", addr, err)
	}

	go func() {
		// blocks on the context's Done channel, once the caller cancels the context,
		// receiving on the Done channel unblocks and the server is closed.
		go func() {
			<-ctx.Done()
			_ = server.Close()
		}()

		buf := make([]byte, 1024)

		for {
			// read from the UDP connection
			n, clientAddr, err := server.ReadFrom(buf)
			if err != nil {
				return
			}

			// write back to the client-address with the same message

			_, err = server.WriteTo(buf[:n], clientAddr)
		}
	}()

	return server.LocalAddr(), nil
}
