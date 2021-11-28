package reads

import (
	"bufio"
	"crypto/rand"
	"io"
	"net"
	"reflect"
	"testing"
)

func TestReadIntoBuffer(t *testing.T) {
	payload := make([]byte, 1<<24) // 16MB
	// generate a random payload for the client to read from the connection
	_, err := rand.Read(payload)
	if err != nil {
		t.Fatal(err)
	}

	// spin-up listener
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// listen for incoming connections in go-routine
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Log(err)
			return
		}
		defer conn.Close()

		// write payload to network connection
		_, err = conn.Write(payload)
		if err != nil {
			t.Error(err)
		}
	}()

	// connect to the address on the network
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// client will read 512KB of data
	buf := make([]byte, 1<<19)

	// keep reading from connection until reaching io.EOF
	for {
		// client will read up to 512KB at a time
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				t.Error(err)
			}
			break
		}
		t.Logf("read %d bytes", n) // buf[:n] is the data read from conn
	}

	conn.Close()
}

func TestScanner(t *testing.T) {
	const payload = "The bigger the interface, the weaker the abstraction."
	// spin-up listener to server payload
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// listen for incoming connections in go-routine
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()

		// write payload to network connection
		_, err = conn.Write([]byte(payload))
		if err != nil {
			t.Error(err)
		}
	}()

	// connect to server-network at address
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// use scanner to read from netowrk connection
	scanner := bufio.NewScanner(conn)
	// have scanner delimit the input at the end of each word
	scanner.Split(bufio.ScanWords)

	var words []string

	for scanner.Scan() {
		words = append(words, scanner.Text())
	}

	err = scanner.Err()
	if err != nil {
		t.Error(err)
	}

	expected := []string{"The", "bigger", "the", "interface,", "the",
		"weaker", "the", "abstraction."}

	if !reflect.DeepEqual(words, expected) {
		t.Fatal("inaccurate scanned word list")
	}

	t.Logf("Scanned words: %#v", words)
}
