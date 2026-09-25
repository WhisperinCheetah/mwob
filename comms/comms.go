// Package comms provides simple length-prefixed, keep-alive TCP
// communication between two peers in a client/server arrangement.
//
// One side calls Start to listen for a peer, the other calls Connect
// to reach it. Both return a *Connection with the same Send/Poll API,
// so the calling code doesn't need to care which side it's on.
package comms

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Connection struct {
	conn net.Conn

	incoming chan []byte
	closed   chan struct{}
	closeErr error
	closeOnce sync.Once

	sendMu sync.Mutex
}

// Connect dials addr as a client, retrying with exponential backoff
// until it succeeds. It blocks until connected.
func Connect(addr string) *Connection {
	conn := connectWithRetry(addr)
	return newConnection(conn)
}

// Start listens on addr and blocks until a single peer connects,
// then returns the resulting Connection. The listener is closed
// once a peer has connected (this package supports one peer at a time).
func Start(addr string) (*Connection, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("comms: listen: %w", err)
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		return nil, fmt.Errorf("comms: accept: %w", err)
	}
	return newConnection(conn), nil
}

// Send writes a single message to the peer. Safe for concurrent use.
func (c *Connection) Send(data []byte) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	return writeMessage(c.conn, data)
}

// Poll returns the next queued incoming message, if any, without blocking.
// ok is false if no message is currently available.
func (c *Connection) Poll() (data []byte, ok bool) {
	select {
	case data := <-c.incoming:
		return data, true
	default:
		return nil, false
	}
}

// Closed returns a channel that is closed when the connection has
// terminated (peer disconnected, read/write error, or explicit Close).
func (c *Connection) Closed() <-chan struct{} {
	return c.closed
}

// Err returns the error that caused the connection to close, if any.
// Only meaningful after Closed() has fired.
func (c *Connection) Err() error {
	return c.closeErr
}

// Close shuts down the connection.
func (c *Connection) Close() error {
	err := c.conn.Close()
	c.closeOnce.Do(func() {
		close(c.closed)
	})
	return err
}

// newConnection wraps an established net.Conn, enables TCP keep-alive,
// and starts the background read loop.
func newConnection(conn net.Conn) *Connection {
	enableKeepAlive(conn)

	c := &Connection{
		conn:     conn,
		incoming: make(chan []byte, 64),
		closed:   make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func (c *Connection) readLoop() {
	for {
		data, err := readMessage(c.conn)
		if err != nil {
			c.closeErr = err
			c.closeOnce.Do(func() {
				close(c.closed)
			})
			return
		}
		c.incoming <- data
	}
}

func enableKeepAlive(conn net.Conn) {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(10 * time.Second)
	}
}

func connectWithRetry(addr string) net.Conn {
	backoff := time.Second
	for {
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			return conn
		}
		fmt.Println("comms: connect failed, retrying:", err)
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// writeMessage writes a length-prefixed message: a 4-byte big-endian
// length followed by that many bytes of payload.
func writeMessage(w io.Writer, data []byte) error {
	length := uint32(len(data))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// readMessage reads a single length-prefixed message written by writeMessage.
func readMessage(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	buf := make([]byte, length)
	_, err := io.ReadFull(r, buf)
	return buf, err
}
