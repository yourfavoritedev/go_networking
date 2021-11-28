package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

var (
	count    = flag.Int("c", 3, "number of pings: <= 0 means forever")
	interval = flag.Duration("i", time.Second, "interval between pings")
	timeout  = flag.Duration("W", 5*time.Second, "time to wait for a reply")
)

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s [options] host:port\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Print("host: port is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	target := flag.Arg(0)
	fmt.Println("PING", target)

	if *count <= 0 {
		fmt.Println("CTRL+C to stop.")
	}

	msg := 0

	// Attempt to make connection to socket while msg is less
	// than count (default is set to 3, but flag can be adjusted)
	for (*count <= 0) || (msg < *count) {
		msg++
		fmt.Printf("Attempt %d: ", msg)

		start := time.Now()
		c, err := net.DialTimeout("tcp", target, *timeout)
		dur := time.Since(start)

		if err != nil {
			fmt.Printf("fail in %s: %v\n", dur, err)
			if nErr, ok := err.(net.Error); !ok || !nErr.Temporary() {
				os.Exit(1)
			}
		} else {
			_ = c.Close()
			// Print the time to establish a TCP socket to a given host and port
			fmt.Println(dur)
			break
		}

		time.Sleep(*interval)
	}
}
