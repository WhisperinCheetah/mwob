package main

import "fmt"
import "net"
// import "mwob/mouse"

func writeMessage(w io.Writer, data []byte) error {
	length := uint32(len(data))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func readMessage(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	buf := make([]byte, length)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func connectWithRetry(addr string) net.Conn {
	backoff := time.Second
	for {
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			return conn
		}
		fmt.Println("connect failed, retrying:", err)
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

package main

import (
	"flag"
	"fmt"
	"time"

	"mwob/comms"
)

func main() {
	/*
	m := mouse.New()
	m.Start()
	defer m.End()

	m.Run()
	*/

	
	mode := flag.String("mode", "server", "server or client")
	addr := flag.String("addr", "localhost:9000", "address to listen on / connect to")
	flag.Parse()

	var conn *comms.Connection

	switch *mode {
	case "server":
		fmt.Println("waiting for peer on", *addr)
		c, err := comms.Start(*addr)
		if err != nil {
			fmt.Println("failed to start:", err)
			return
		}
		conn = c
	case "client":
		fmt.Println("connecting to", *addr)
		conn = comms.Connect(*addr)
	default:
		fmt.Println("unknown mode:", *mode)
		return
	}

	fmt.Println("connected")
	defer conn.Close()

	// Send a message every 2 seconds.
	go func() {
		i := 0
		for {
			select {
			case <-conn.Closed():
				return
			default:
			}
			msg := fmt.Sprintf("hello #%d from %s", i, *mode)
			if err := conn.Send([]byte(msg)); err != nil {
				fmt.Println("send error:", err)
				return
			}
			i++
			time.Sleep(2 * time.Second)
		}
	}()

	// Poll for incoming messages.
	for {
		select {
		case <-conn.Closed():
			fmt.Println("connection closed:", conn.Err())
			return
		default:
		}

		if data, ok := conn.Poll(); ok {
			fmt.Println("received:", string(data))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

