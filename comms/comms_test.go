package comms

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

// --- Unit tests: message framing -------------------------------------------

func TestWriteReadMessage_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("hello")},
		{"binary", []byte{0x00, 0xFF, 0x10, 0x00, 0x01}},
		{"long", bytes.Repeat([]byte("x"), 10000)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			if err := writeMessage(&buf, tc.data); err != nil {
				t.Fatalf("writeMessage returned error: %v", err)
			}

			got, err := readMessage(&buf)
			if err != nil {
				t.Fatalf("readMessage returned error: %v", err)
			}

			if !bytes.Equal(got, tc.data) {
				t.Fatalf("round trip mismatch: got %v, want %v", got, tc.data)
			}
		})
	}
}

func TestReadMessage_TruncatedLength(t *testing.T) {
	// Only 2 bytes written, but a uint32 length prefix needs 4.
	buf := bytes.NewBuffer([]byte{0x00, 0x01})

	_, err := readMessage(buf)
	if err == nil {
		t.Fatal("expected error reading truncated length prefix, got nil")
	}
}

func TestReadMessage_TruncatedPayload(t *testing.T) {
	var buf bytes.Buffer
	// Claim a 10-byte payload but only write 3.
	if err := writeMessage(&buf, []byte("abcdefghij")); err != nil {
		t.Fatalf("setup: writeMessage failed: %v", err)
	}
	truncated := bytes.NewBuffer(buf.Bytes()[:4+3]) // length prefix + 3 payload bytes

	_, err := readMessage(truncated)
	if err == nil {
		t.Fatal("expected error reading truncated payload, got nil")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got: %v", err)
	}
}

func TestWriteMessage_MultipleMessagesSequential(t *testing.T) {
	var buf bytes.Buffer
	msgs := [][]byte{[]byte("first"), []byte("second"), []byte("third")}

	for _, m := range msgs {
		if err := writeMessage(&buf, m); err != nil {
			t.Fatalf("writeMessage failed: %v", err)
		}
	}

	for _, want := range msgs {
		got, err := readMessage(&buf)
		if err != nil {
			t.Fatalf("readMessage failed: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

// --- Integration tests: Start/Connect over real TCP -------------------------

// waitForMessage polls Poll() until a message arrives or the timeout expires.
func waitForMessage(t *testing.T, c *Connection, timeout time.Duration) []byte {
	t.Helper()
	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		if data, ok := c.Poll(); ok {
			return data
		}
		select {
		case <-ticker.C:
			continue
		case <-deadline:
			t.Fatal("timed out waiting for message")
			return nil
		}
	}
}

func TestStartConnect_SendPollRoundTrip(t *testing.T) {
	const addr = "127.0.0.1:19323"

	serverCh := make(chan *Connection, 1)
	serverErrCh := make(chan error, 1)
	go func() {
		conn, err := Start(addr)
		if err != nil {
			serverErrCh <- err
			return
		}
		serverCh <- conn
	}()

	client := Connect(addr) // Connect retries internally until the listener is up
	defer client.Close()

	var server *Connection
	select {
	case server = <-serverCh:
	case err := <-serverErrCh:
		t.Fatalf("Start failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to accept connection")
	}
	defer server.Close()

	// client -> server
	if err := client.Send([]byte("ping")); err != nil {
		t.Fatalf("client.Send failed: %v", err)
	}
	got := waitForMessage(t, server, 2*time.Second)
	if string(got) != "ping" {
		t.Fatalf("server got %q, want %q", got, "ping")
	}

	// server -> client
	if err := server.Send([]byte("pong")); err != nil {
		t.Fatalf("server.Send failed: %v", err)
	}
	got = waitForMessage(t, client, 2*time.Second)
	if string(got) != "pong" {
		t.Fatalf("client got %q, want %q", got, "pong")
	}
}

func TestConnection_PollReturnsFalseWhenEmpty(t *testing.T) {
	const addr = "127.0.0.1:19324"

	serverCh := make(chan *Connection, 1)
	go func() {
		conn, err := Start(addr)
		if err == nil {
			serverCh <- conn
		}
	}()

	client := Connect(addr)
	defer client.Close()

	server := <-serverCh
	defer server.Close()

	if data, ok := client.Poll(); ok {
		t.Fatalf("expected no message, got %q", data)
	}
}

func TestConnection_CloseSignalsClosed(t *testing.T) {
	const addr = "127.0.0.1:19325"

	serverCh := make(chan *Connection, 1)
	go func() {
		conn, err := Start(addr)
		if err == nil {
			serverCh <- conn
		}
	}()

	client := Connect(addr)
	server := <-serverCh
	defer server.Close()

	client.Close()

	select {
	case <-client.Closed():
		// expected
	case <-time.After(1 * time.Second):
		t.Fatal("Closed() channel did not fire after Close()")
	}
}
