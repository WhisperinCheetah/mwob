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

