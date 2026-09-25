package main

import (
	"bytes"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"time"

	"mwob/comms"
	"mwob/mouse"
)

func master(conn *comms.Connection) {
	m := mouse.New()
	m.Start()
	defer m.End()
	go m.Run()

	var buf bytes.Buffer
	for {
		current, _, changed := m.Poll()

		if changed {
			err := gob.NewEncoder(&buf).Encode(current)

			if err != nil {
				log.Fatal(err)
			}

			select {
			case <-conn.Closed():
				return
			default:
			}

			fmt.Println(current)
			msg := buf.Bytes()
			if err := conn.Send(msg); err != nil {
				fmt.Println("send error:", err)
				return
			}
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func slave(conn *comms.Connection) {
	// Poll for incoming messages.
	var s mouse.State
	for {
		select {
		case <-conn.Closed():
			fmt.Println("connection closed:", conn.Err())
			return
		default:
		}

		if data, ok := conn.Poll(); ok {
			err := gob.NewDecoder(bytes.NewReader(data)).Decode(&s)

			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(s)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func main() {
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

	master(conn)

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
