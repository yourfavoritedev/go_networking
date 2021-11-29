package echo

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"
)

func TestEchoServerUDP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// initiate UDP server connection
	serverAddr, err := echoServerUDP(ctx, "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	// initiate UDP client connection
	client, err := net.ListenPacket("udp", "127.0.0.1:")
	defer client.Close()

	msg := []byte("ping")
	// write from client to server
	_, err = client.WriteTo(msg, serverAddr)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1024)
	// read message sent to client from server
	n, addr, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	if addr.String() != serverAddr.String() {
		t.Fatalf("received reply from %q instead of %q", addr, serverAddr)
	}

	if !bytes.Equal(msg, buf[:n]) {
		t.Errorf("expected reply %q, actual reply: %q", msg, buf[:n])
	}
}

func TestListenPacketUDP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// server UDP connection
	serverAddr, err := echoServerUDP(ctx, "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	// client UDP connection
	client, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// interloper UDP connection
	interloper, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	interrupt := []byte("pardon me")
	// interloper to interrupt the client connection by writing to it
	n, err := interloper.WriteTo(interrupt, client.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}
	_ = interloper.Close()

	// validate how many bytes were written by the interloper to the client
	if l := len(interrupt); l != n {
		t.Fatalf("wrote %d bytes of %d", n, l)
	}

	ping := []byte("ping")
	// client write to server (the server will respond back immediately with the same message)
	_, err = client.WriteTo(ping, serverAddr)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1024)
	// client to read 1st message on connection
	n, addr, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	// validate that the first message in the client connection is the message from the interloper
	if !bytes.Equal(interrupt, buf[:n]) {
		t.Errorf("expected reply %q, actual reply %q", interrupt, buf[:n])
	}

	// validate that the address of the first message in the client connection is from the interloper
	if addr.String() != interloper.LocalAddr().String() {
		t.Errorf("expected message from: %q, actual sender is %q",
			interloper.LocalAddr().String(), addr.String())
	}

	// client to read 2nd message on connection
	n, addr, err = client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	// validate that the second message in the client connection is the message from the server
	if !bytes.Equal(ping, buf[:n]) {
		t.Errorf("expected reply %q, actual reply %q", ping, buf[:n])
	}

	// validate that the address of second message in the client connection is from the server
	if addr.String() != serverAddr.String() {
		t.Errorf("expected message from: %q, actual sender is %q",
			serverAddr.String(), addr.String())
	}
}

func TestDialUDP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// initiate UDP server
	serverAddr, err := echoServerUDP(ctx, "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	// dial UDP server using 'udp' to emulate a TCP connection
	client, err := net.Dial("udp", serverAddr.String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// interloper UDP connection
	interloper, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	interrupt := []byte("pardon me")
	// interloper attempts to interrupt the client connection by writing to it
	n, err := interloper.WriteTo(interrupt, client.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}
	_ = interloper.Close()

	// validate how many bytes were written by the interloper to the client
	if l := len(interrupt); l != n {
		t.Fatalf("wrote %d bytes of %d", n, l)
	}

	ping := []byte("ping")
	// client to write directly to UDP server
	_, err = client.Write(ping)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1024)
	// client to read loaded buffer on connection
	n, err = client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	// validate that the message on the client buffer is from the server and not the interloper
	if !bytes.Equal(ping, buf[:n]) {
		t.Errorf("expected reply %q, actual reply %q",
			ping, buf[:n])
	}

	// advance deadline to 2 more seconds to try reading another message
	err = client.SetDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		t.Fatal(err)
	}

	// attempt to read another message while knowing the connection will only
	// be up for 2 more seconds. Expect timeout.
	_, err = client.Read(buf)
	if err == nil {
		t.Fatal("unexpected packet")
	}
}
