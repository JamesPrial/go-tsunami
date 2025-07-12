package server

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jamesprial/go-tsunami/protocol/common"
)

// Test helper to create UDP listener that captures packets
type udpCapture struct {
	packets [][]byte
	conn    *net.UDPConn
	done    chan struct{}
	mutex   sync.RWMutex // Add mutex for thread safety
}

func newUDPCapture() (*udpCapture, int, error) {
	// Listen on port 0 to let the OS choose a free port
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, 0, err
	}

	capture := &udpCapture{
		packets: make([][]byte, 0),
		conn:    conn,
		done:    make(chan struct{}),
	}

	// Get the port that was actually assigned
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	port := localAddr.Port

	// Start capturing packets in background
	go capture.captureLoop()

	return capture, port, nil
}

func (u *udpCapture) captureLoop() {
	buffer := make([]byte, 65536) // Large buffer for any packet size

	for {
		select {
		case <-u.done:
			return
		default:
			u.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, err := u.conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Keep trying
				}
				return // Real error, stop
			}

			// Store copy of packet data with thread safety
			packet := make([]byte, n)
			copy(packet, buffer[:n])

			u.mutex.Lock()
			u.packets = append(u.packets, packet)
			u.mutex.Unlock()
		}
	}
}

func (u *udpCapture) stop() {
	close(u.done)
	u.conn.Close()
}

func (u *udpCapture) getPackets() [][]byte {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	// Return a copy of the packets slice to prevent race conditions
	result := make([][]byte, len(u.packets))
	copy(result, u.packets)
	return result
}

// testHarness manages a running server and a client connection for integration tests.
type testHarness struct {
	t        *testing.T
	server   *Server
	client   net.Conn
	listener net.Listener
}

// newTestHarness creates and starts a real server for testing.
func newTestHarness(t *testing.T, files map[string][]byte) *testHarness {
	fs := fstest.MapFS{}
	for name, content := range files {
		fs[name] = &fstest.MapFile{Data: content}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Start server on a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on a port: %v", err)
	}

	server := NewServerWithLogger(listener, &fs, logger)

	// Run the server in the background
	go func() {
		// Listen() is a blocking call, so we run it in a goroutine.
		// We expect it to return an error when we close the listener.
		if err := server.Listen(); err != nil && err != net.ErrClosed {
			t.Errorf("Server failed to listen: %v", err)
		}
	}()

	// Connect a client to the server
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect to the server: %v", err)
	}

	return &testHarness{
		t:        t,
		server:   server,
		client:   client,
		listener: listener,
	}
}

// close stops the server and client.
func (h *testHarness) close() {
	h.client.Close()
	h.listener.Close() // This will stop the server's Listen() loop.
}

// sendCommand sends a command to the server.
func (h *testHarness) sendCommand(cmd common.Command) {
	data, err := cmd.MarshalBinary()
	if err != nil {
		h.t.Fatalf("Failed to marshal command: %v", err)
	}
	_, err = h.client.Write(data)
	if err != nil {
		h.t.Fatalf("Failed to write command to server: %v", err)
	}
}

// readResponse reads a response from the server.
func (h *testHarness) readResponse() common.Command {
	h.client.SetReadDeadline(time.Now().Add(2 * time.Second))
	scanner := bufio.NewScanner(h.client)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			h.t.Fatalf("Failed to read response from server: %v", err)
		}
		h.t.Fatalf("Failed to read response from server: empty scan")
	}

	line := scanner.Bytes()
	cmd, err := common.UnmarshalCommand(line)
	if err != nil {
		h.t.Fatalf("Failed to unmarshal server response: %v (line: %q)", err, string(line))
	}
	return cmd
}

func TestIntegrationFileTransmission(t *testing.T) {
	// Create test file (20 bytes)
	testData := []byte("0123456789abcdefghij")
	h := newTestHarness(t, map[string][]byte{"test.txt": testData})
	defer h.close()

	// Set up UDP capture on a random port
	capture, udpPort, err := newUDPCapture()
	if err != nil {
		t.Fatalf("Failed to create UDP capture: %v", err)
	}
	defer capture.stop()

	// Send GET command
	getCmd := &common.GetCommand{
		Filename:  "test.txt",
		Blocksize: 10,
		UdpPort:   uint64(udpPort),
	}
	h.sendCommand(getCmd)

	// Verify OK response
	resp := h.readResponse()
	okCmd, ok := resp.(*common.OkCommand)
	if !ok {
		t.Fatalf("Expected OK command, got %T", resp)
	}
	if okCmd.Filesize != uint64(len(testData)) {
		t.Errorf("Expected filesize %d, got %d", len(testData), okCmd.Filesize)
	}

	// Give transmission time to complete
	time.Sleep(200 * time.Millisecond)

	// Verify UDP packets
	packets := capture.getPackets()
	if len(packets) != 2 {
		t.Fatalf("Expected 2 packets, got %d", len(packets))
	}
	if len(packets[0]) != 18 { // 8 bytes header + 10 bytes data
		t.Errorf("Expected packet 1 length 18, got %d", len(packets[0]))
	}
	if len(packets[1]) != 18 { // 8 bytes header + 10 bytes data
		t.Errorf("Expected packet 2 length 18, got %d", len(packets[1]))
	}
}

func TestIntegrationRetrRestDoneLifecycle(t *testing.T) {
	testData := bytes.Repeat([]byte("x"), 100) // 100 bytes
	h := newTestHarness(t, map[string][]byte{"lifecycle.txt": testData})
	defer h.close()

	capture, udpPort, err := newUDPCapture()
	if err != nil {
		t.Fatalf("Failed to create UDP capture: %v", err)
	}
	defer capture.stop()

	// 1. GET command
	h.sendCommand(&common.GetCommand{
		Filename:  "lifecycle.txt",
		Blocksize: 10,
		UdpPort:   uint64(udpPort),
	})
	resp := h.readResponse()
	if _, ok := resp.(*common.OkCommand); !ok {
		t.Fatalf("Expected OK command after GET, got %T", resp)
	}
	time.Sleep(200 * time.Millisecond) // Allow initial transmission

	// 2. RETR command
	h.sendCommand(&common.RetrCommand{BlockIndex: 5})
	time.Sleep(100 * time.Millisecond) // Allow retransmission

	// 3. REST command
	h.sendCommand(&common.RestCommand{BlockIndex: 8})
	time.Sleep(100 * time.Millisecond) // Allow restart

	// 4. DONE command
	h.sendCommand(&common.DoneCommand{})
	// Give the server a moment to process the DONE command and clean up.
	time.Sleep(100 * time.Millisecond)

	// Verification
	packets := capture.getPackets()
	// Initial: 10 blocks. RETR: 1 block. REST: 2 blocks (8, 9). Total: 13
	if len(packets) < 13 {
		t.Errorf("Expected at least 13 packets, got %d", len(packets))
	}

	// Check that the server cleaned up the transmission state
	h.server.transmissionsMutex.RLock()
	if len(h.server.transmissions) != 0 {
		t.Errorf("Server did not clean up transmission state after DONE")
	}
	h.server.transmissionsMutex.RUnlock()
}

func TestIntegrationConcurrentTransfers(t *testing.T) {
	t.Parallel()
	// Setup a single server with multiple files
	files := map[string][]byte{
		"file1.txt": bytes.Repeat([]byte("A"), 200),
		"file2.txt": bytes.Repeat([]byte("B"), 200),
	}
	h := newTestHarness(t, files)
	defer h.close()

	var wg sync.WaitGroup
	numClients := 2
	wg.Add(numClients)

	runClient := func(filename string) {
		defer wg.Done()
		client, err := net.Dial("tcp", h.listener.Addr().String())
		if err != nil {
			t.Errorf("Concurrent client failed to connect: %v", err)
			return
		}
		defer client.Close()

		capture, udpPort, err := newUDPCapture()
		if err != nil {
			t.Errorf("Failed to create UDP capture: %v", err)
			return
		}
		defer capture.stop()

		// Send GET
		getCmd := &common.GetCommand{Filename: filename, Blocksize: 20, UdpPort: uint64(udpPort)}
		data, _ := getCmd.MarshalBinary()
		client.Write(data)

		// Read OK
		scanner := bufio.NewScanner(client)
		if !scanner.Scan() {
			t.Errorf("Failed to read OK response for %s", filename)
			return
		}

		time.Sleep(300 * time.Millisecond) // Wait for transmission

		// Verify packets
		packets := capture.getPackets()
		if len(packets) != 10 {
			t.Errorf("Expected 10 packets for %s, got %d", filename, len(packets))
		}
	}

	go runClient("file1.txt")
	go runClient("file2.txt")

	wg.Wait()
}
