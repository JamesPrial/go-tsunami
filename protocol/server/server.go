package server

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/jamesprial/go-tsunami/protocol/common"
)

// transmissionState holds state for an active file transmission
type transmissionState struct {
	filename    string
	blockSize   uint64
	totalBlocks uint64
	sentBlocks  map[uint64]bool
	fileHandle  fs.File
	clientAddr  *net.UDPAddr
	udpConn     *net.UDPConn
	mutex       sync.RWMutex
}

// Server represents a Tsunami file server with structured logging
type Server struct {
	FileSystem fs.FS
	listener   net.Listener
	logger     *slog.Logger
	// Active transmissions per client IP
	transmissions      map[string]*transmissionState
	transmissionsMutex sync.RWMutex
}

// clientSession holds state for a single client connection with contextual logging
type clientSession struct {
	server     *Server
	conn       net.Conn
	writer     *bufio.Writer
	scanner    *bufio.Scanner
	clientAddr *net.TCPAddr
	logger     *slog.Logger
}

// Logging helper functions for consistent error handling

// logError logs an error with structured information, handling both ServerError and generic errors
func logError(logger *slog.Logger, message string, err error) {
	if serverErr, ok := err.(*ServerError); ok {
		logger.Error(message,
			slog.String("operation", serverErr.Operation()),
			slog.String("error_code", serverErr.Code().String()),
			slog.String("client", serverErr.Client()),
			slog.String("error", serverErr.Error()))
	} else {
		logger.Error(message,
			slog.String("error", err.Error()))
	}
}

// logSessionError logs session-specific errors with client context
func (cs *clientSession) logError(message string, err error) {
	logError(cs.logger, message, err)
}

// logServerError logs server-level errors
func (s *Server) logError(message string, err error) {
	logError(s.logger, message, err)
}

// NewServer creates a new Tsunami server
func NewServer(listener net.Listener, filesystem fs.FS) *Server {
	if filesystem == nil {
		filesystem = os.DirFS(".")
	}

	// Create structured logger with default configuration
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &Server{
		listener:      listener,
		FileSystem:    filesystem,
		logger:        logger,
		transmissions: make(map[string]*transmissionState),
	}
}

// NewServerWithLogger creates a new Tsunami server with custom logger
func NewServerWithLogger(listener net.Listener, filesystem fs.FS, logger *slog.Logger) *Server {
	if filesystem == nil {
		filesystem = os.DirFS(".")
	}

	return &Server{
		listener:      listener,
		FileSystem:    filesystem,
		logger:        logger,
		transmissions: make(map[string]*transmissionState),
	}
}

func (s *Server) GetFileSize(filepath string) (int64, error) {
	file, err := s.FileSystem.Open(filepath)
	if err != nil {
		return -1, newFileError("open file", filepath, err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return -1, newFileError("stat file", filepath, err)
	}
	return stat.Size(), nil
}

// Listen starts the server and handles incoming connections
func (s *Server) Listen() error {
	s.logger.Info("Tsunami server started",
		slog.String("address", s.listener.Addr().String()))

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// If the listener was closed, this is a graceful shutdown.
			if err == net.ErrClosed {
				return nil
			}
			s.logError("Failed to accept connection", err)
			continue
		}

		// Handle each connection in a separate goroutine for concurrent transfers
		go s.handleConnection(conn)
	}
}

// handleConnection processes a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Get client address as proper TCP address
	clientAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		s.logger.Error("Invalid client address type",
			slog.String("address_type", fmt.Sprintf("%T", conn.RemoteAddr())))
		return
	}

	clientIP := clientAddr.IP.String()
	// Ensure that any transmission state is cleaned up when the client disconnects.
	defer s.removeTransmissionState(clientIP)

	// Create session logger with client context
	sessionLogger := s.logger.With(
		slog.String("client_ip", clientIP),
		slog.Int("client_port", clientAddr.Port))

	sessionLogger.Info("Client connected")

	// Create client session with all necessary context
	session := &clientSession{
		server:     s,
		conn:       conn,
		writer:     bufio.NewWriter(conn),
		scanner:    bufio.NewScanner(conn),
		clientAddr: clientAddr,
		logger:     sessionLogger,
	}

	// Process commands for this session
	if err := session.handleCommands(); err != nil {
		session.logError("Session error", err)
	}

	sessionLogger.Info("Client disconnected")
}

// handleCommands processes commands for a client session
func (cs *clientSession) handleCommands() error {
	clientIP := cs.clientAddr.IP.String()

	for cs.scanner.Scan() {
		line := cs.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse the command
		cmd, err := common.UnmarshalCommand(line)
		if err != nil {
			protocolErr := newProtocolError("parse command", clientIP, err)
			if sendErr := cs.sendError(protocolErr.Error()); sendErr != nil {
				return newNetworkError("send error response", clientIP, sendErr)
			}
			continue
		}

		// Handle the command with full session context
		if err := cs.handleCommand(cmd); err != nil {
			if sendErr := cs.sendError(fmt.Sprintf("Command failed: %v", err)); sendErr != nil {
				return newNetworkError("send error response", clientIP, sendErr)
			}
			continue
		}
	}

	if err := cs.scanner.Err(); err != nil {
		return newNetworkError("connection scan", clientIP, err)
	}

	return nil
}

// handleCommand processes different command types with session context
func (cs *clientSession) handleCommand(cmd common.Command) error {
	switch c := cmd.(type) {
	case *common.GetCommand:
		return cs.handleGetCommand(c)
	case *common.RetrCommand:
		return cs.handleRetrCommand(c)
	case *common.RestCommand:
		return cs.handleRestCommand(c)
	case *common.DoneCommand:
		return cs.handleDoneCommand(c)
	default:
		return fmt.Errorf("unsupported command type: %T", cmd)
	}
}

// handleGetCommand processes GET requests
func (cs *clientSession) handleGetCommand(cmd *common.GetCommand) error {
	cs.logger.Info("GET request received",
		slog.String("filename", cmd.Filename),
		slog.Uint64("blocksize", cmd.Blocksize),
		slog.Uint64("udp_port", cmd.UdpPort))

	// Check if file exists and get its size
	filesize, err := cs.server.GetFileSize(cmd.Filename)
	if err != nil {
		fileErr := newFileError("get file size", cmd.Filename, err)
		cs.logger.Warn("File not found",
			slog.String("filename", cmd.Filename),
			slog.String("error", err.Error()))
		return cs.sendError(fileErr.Error())
	}

	cs.logger.Info("File found",
		slog.String("filename", cmd.Filename),
		slog.Int64("size", filesize))

	// Send OK response with file size
	okCmd := &common.OkCommand{Filesize: uint64(filesize)}
	data, err := okCmd.MarshalBinary()
	if err != nil {
		return newProtocolError("marshal OK command", cs.clientAddr.IP.String(), err)
	}

	_, err = cs.writer.Write(data)
	if err != nil {
		return newNetworkError("write OK response", cs.clientAddr.IP.String(), err)
	}
	if err := cs.writer.Flush(); err != nil {
		return newNetworkError("flush OK response", cs.clientAddr.IP.String(), err)
	}

	// Start UDP file transmission in the background.
	// The transmission will run concurrently, allowing this handler to return
	// and the server to process other commands (like RETR or DONE).
	go func() {
		if err := cs.startFileTransmission(cmd); err != nil {
			// Log the error. Cleanup is handled by the defer in handleConnection.
			cs.logError("File transmission failed", err)
		}
	}()

	return nil
}

// handleRetrCommand processes RETR requests (block retransmission)
func (cs *clientSession) handleRetrCommand(cmd *common.RetrCommand) error {
	clientIP := cs.clientAddr.IP.String()
	cs.logger.Debug("RETR request received",
		slog.Uint64("block_index", cmd.BlockIndex),
		slog.String("client_ip", clientIP))

	// Find active transmission for this client
	transmission := cs.server.getTransmissionState(clientIP)
	if transmission == nil {
		cs.logger.Warn("No active transmission found for RETR request",
			slog.String("client_ip", clientIP))
		return cs.sendError("No active transmission")
	}

	// Retransmit specific block
	if err := transmission.retransmitBlock(cmd.BlockIndex); err != nil {
		cs.logger.Error("Block retransmission failed",
			slog.Uint64("block_index", cmd.BlockIndex),
			slog.String("error", err.Error()))
		return cs.sendError(fmt.Sprintf("Retransmission failed: %v", err))
	}

	cs.logger.Info("Block retransmitted successfully",
		slog.Uint64("block_index", cmd.BlockIndex))
	return nil
}

// handleRestCommand processes REST requests (restart transmission)
func (cs *clientSession) handleRestCommand(cmd *common.RestCommand) error {
	clientIP := cs.clientAddr.IP.String()
	cs.logger.Debug("REST request received",
		slog.Uint64("block_index", cmd.BlockIndex),
		slog.String("client_ip", clientIP))

	// Find active transmission for this client
	transmission := cs.server.getTransmissionState(clientIP)
	if transmission == nil {
		cs.logger.Warn("No active transmission found for REST request",
			slog.String("client_ip", clientIP))
		return cs.sendError("No active transmission")
	}

	// Restart from specified block
	if err := transmission.restartFromBlock(cmd.BlockIndex); err != nil {
		cs.logger.Error("Transmission restart failed",
			slog.Uint64("block_index", cmd.BlockIndex),
			slog.String("error", err.Error()))
		return cs.sendError(fmt.Sprintf("Restart failed: %v", err))
	}

	cs.logger.Info("Transmission restarted successfully",
		slog.Uint64("block_index", cmd.BlockIndex))
	return nil
}

// handleDoneCommand processes DONE requests
func (cs *clientSession) handleDoneCommand(cmd *common.DoneCommand) error {
	clientIP := cs.clientAddr.IP.String()
	cs.logger.Info("DONE request received - transfer complete",
		slog.String("client_ip", clientIP))

	// Clean up transmission state for this client
	cs.server.removeTransmissionState(clientIP)
	cs.logger.Debug("Transmission state cleaned up",
		slog.String("client_ip", clientIP))

	return nil
}

// startFileTransmission begins UDP file transmission using transmission state management
func (cs *clientSession) startFileTransmission(cmd *common.GetCommand) error {
	clientIP := cs.clientAddr.IP.String()

	cs.logger.Info("Starting UDP transmission",
		slog.String("filename", cmd.Filename))

	// Create transmission state for this client
	state, err := cs.server.createTransmissionState(clientIP, cmd)
	if err != nil {
		return err
	}

	cs.logger.Info("Starting block transmission",
		slog.Uint64("total_blocks", state.totalBlocks),
		slog.Uint64("block_size", state.blockSize),
		slog.String("filename", state.filename))

	// Send blocks via UDP using transmission state
	buffer := make([]byte, state.blockSize)
	for blockIndex := uint64(0); blockIndex < state.totalBlocks; blockIndex++ {
		// Lock the state for reading the file and sending the block
		state.mutex.Lock()

		n, err := state.fileHandle.Read(buffer)
		if err != nil && err != io.EOF {
			state.mutex.Unlock()
			return newTransmissionError("read file", clientIP, blockIndex, err)
		}

		if n == 0 {
			state.mutex.Unlock()
			break
		}

		// Create block packet: 8 bytes block index + data
		blockData := make([]byte, 8+n)
		// Write block index (big endian)
		blockData[0] = byte(blockIndex >> 56)
		blockData[1] = byte(blockIndex >> 48)
		blockData[2] = byte(blockIndex >> 40)
		blockData[3] = byte(blockIndex >> 32)
		blockData[4] = byte(blockIndex >> 24)
		blockData[5] = byte(blockIndex >> 16)
		blockData[6] = byte(blockIndex >> 8)
		blockData[7] = byte(blockIndex)

		// Copy file data
		copy(blockData[8:], buffer[:n])

		// Send block using transmission state
		_, err = state.udpConn.Write(blockData)
		if err != nil {
			state.mutex.Unlock()
			return newTransmissionError("send block", clientIP, blockIndex, err)
		}

		// Mark block as sent
		state.sentBlocks[blockIndex] = true

		// Unlock after the operation is complete for this block
		state.mutex.Unlock()

		if blockIndex%100 == 0 {
			cs.logger.Debug("Block transmission progress",
				slog.Uint64("blocks_sent", blockIndex),
				slog.Uint64("total_blocks", state.totalBlocks))
		}
	}

	cs.logger.Info("File transmission completed",
		slog.Uint64("blocks_sent", state.totalBlocks),
		slog.String("filename", state.filename))

	return nil
}

// Transmission state management methods

// createTransmissionState creates a new transmission state for a client
func (s *Server) createTransmissionState(clientIP string, cmd *common.GetCommand) (*transmissionState, error) {
	// Open file for transmission
	file, err := s.FileSystem.Open(cmd.Filename)
	if err != nil {
		return nil, newFileError("open file", cmd.Filename, err)
	}

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, newFileError("get file info", cmd.Filename, err)
	}

	fileSize := uint64(fileInfo.Size())
	totalBlocks := (fileSize + cmd.Blocksize - 1) / cmd.Blocksize

	// Create UDP address
	clientUDPAddr := &net.UDPAddr{
		IP:   net.ParseIP(clientIP),
		Port: int(cmd.UdpPort),
	}

	// Create UDP connection
	udpConn, err := net.DialUDP("udp", nil, clientUDPAddr)
	if err != nil {
		file.Close()
		return nil, newNetworkError("create UDP connection", clientIP, err)
	}

	state := &transmissionState{
		filename:    cmd.Filename,
		blockSize:   cmd.Blocksize,
		totalBlocks: totalBlocks,
		sentBlocks:  make(map[uint64]bool),
		fileHandle:  file,
		clientAddr:  clientUDPAddr,
		udpConn:     udpConn,
	}

	s.transmissionsMutex.Lock()
	s.transmissions[clientIP] = state
	s.transmissionsMutex.Unlock()

	return state, nil
}

// getTransmissionState retrieves the transmission state for a client
func (s *Server) getTransmissionState(clientIP string) *transmissionState {
	s.transmissionsMutex.RLock()
	defer s.transmissionsMutex.RUnlock()
	return s.transmissions[clientIP]
}

// removeTransmissionState removes the transmission state for a client
func (s *Server) removeTransmissionState(clientIP string) {
	s.transmissionsMutex.Lock()
	defer s.transmissionsMutex.Unlock()

	if state, exists := s.transmissions[clientIP]; exists {
		// Close resources
		if state.fileHandle != nil {
			state.fileHandle.Close()
		}
		if state.udpConn != nil {
			state.udpConn.Close()
		}
		delete(s.transmissions, clientIP)
	}
}

// transmissionState methods

// retransmitBlock retransmits a specific block
func (ts *transmissionState) retransmitBlock(blockIndex uint64) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if blockIndex >= ts.totalBlocks {
		return fmt.Errorf("block index %d out of range (total blocks: %d)", blockIndex, ts.totalBlocks)
	}

	// Seek to the correct position in the file
	offset := int64(blockIndex * ts.blockSize)
	seeker, ok := ts.fileHandle.(io.Seeker)
	if !ok {
		return fmt.Errorf("file handle does not support seeking")
	}

	_, err := seeker.Seek(offset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek to block %d: %w", blockIndex, err)
	}

	// Read the block data
	buffer := make([]byte, ts.blockSize)
	n, err := ts.fileHandle.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("read block %d: %w", blockIndex, err)
	}

	if n == 0 {
		return fmt.Errorf("no data to retransmit for block %d", blockIndex)
	}

	// Create block packet
	blockData := make([]byte, 8+n)
	// Write block index (big endian)
	blockData[0] = byte(blockIndex >> 56)
	blockData[1] = byte(blockIndex >> 48)
	blockData[2] = byte(blockIndex >> 40)
	blockData[3] = byte(blockIndex >> 32)
	blockData[4] = byte(blockIndex >> 24)
	blockData[5] = byte(blockIndex >> 16)
	blockData[6] = byte(blockIndex >> 8)
	blockData[7] = byte(blockIndex)

	// Copy file data
	copy(blockData[8:], buffer[:n])

	// Send block
	_, err = ts.udpConn.Write(blockData)
	if err != nil {
		return fmt.Errorf("send block %d: %w", blockIndex, err)
	}

	// Mark as sent
	ts.sentBlocks[blockIndex] = true
	return nil
}

// restartFromBlock restarts transmission from a specific block
func (ts *transmissionState) restartFromBlock(blockIndex uint64) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if blockIndex >= ts.totalBlocks {
		return fmt.Errorf("block index %d out of range (total blocks: %d)", blockIndex, ts.totalBlocks)
	}

	// Clear sent blocks from the restart point onwards
	for i := blockIndex; i < ts.totalBlocks; i++ {
		delete(ts.sentBlocks, i)
	}

	// Seek to the correct position in the file
	offset := int64(blockIndex * ts.blockSize)
	seeker, ok := ts.fileHandle.(io.Seeker)
	if !ok {
		return fmt.Errorf("file handle does not support seeking")
	}

	_, err := seeker.Seek(offset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek to block %d: %w", blockIndex, err)
	}

	// Transmit remaining blocks
	buffer := make([]byte, ts.blockSize)
	for currentBlock := blockIndex; currentBlock < ts.totalBlocks; currentBlock++ {
		n, err := ts.fileHandle.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("read block %d: %w", currentBlock, err)
		}

		if n == 0 {
			break
		}

		// Create block packet
		blockData := make([]byte, 8+n)
		// Write block index (big endian)
		blockData[0] = byte(currentBlock >> 56)
		blockData[1] = byte(currentBlock >> 48)
		blockData[2] = byte(currentBlock >> 40)
		blockData[3] = byte(currentBlock >> 32)
		blockData[4] = byte(currentBlock >> 24)
		blockData[5] = byte(currentBlock >> 16)
		blockData[6] = byte(currentBlock >> 8)
		blockData[7] = byte(currentBlock)

		// Copy file data
		copy(blockData[8:], buffer[:n])

		// Send block
		_, err = ts.udpConn.Write(blockData)
		if err != nil {
			return fmt.Errorf("send block %d: %w", currentBlock, err)
		}

		// Mark as sent
		ts.sentBlocks[currentBlock] = true
	}

	return nil
}

// markBlockSent marks a block as sent
func (ts *transmissionState) markBlockSent(blockIndex uint64) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	ts.sentBlocks[blockIndex] = true
}

// isBlockSent checks if a block has been sent
func (ts *transmissionState) isBlockSent(blockIndex uint64) bool {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()
	return ts.sentBlocks[blockIndex]
}

// sendError sends an error response to the client
func (cs *clientSession) sendError(message string) error {
	errCmd := &common.ErrCommand{Msg: message}
	data, err := errCmd.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = cs.writer.Write(data)
	if err != nil {
		return err
	}
	return cs.writer.Flush()
}
