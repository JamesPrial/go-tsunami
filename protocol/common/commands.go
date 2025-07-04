package common

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// TcpInstruction represents a Tsunami protocol command type
type TcpInstruction string

const (
	GET     TcpInstruction = "GET"
	RETR    TcpInstruction = "RETR"
	OK      TcpInstruction = "OK"
	ERR     TcpInstruction = "ERR"
	REST    TcpInstruction = "REST"
	DONE    TcpInstruction = "DONE"
	INVALID TcpInstruction = "INVALID"
)

// String returns the string representation of the instruction
func (t TcpInstruction) String() string {
	return string(t)
}

// Command represents a Tsunami protocol command
type Command interface {
	MarshalBinary() (data []byte, err error)
	UnmarshalBinary(data []byte) error
	Instruction() TcpInstruction
}

// ParseTcpInstruction parses a string into a TcpInstruction
func ParseTcpInstruction(str string) (TcpInstruction, error) {
	str = strings.TrimSpace(str)
	str = strings.ToUpper(str)
	switch str {
	case "GET":
		return GET, nil
	case "RETR":
		return RETR, nil
	case "OK":
		return OK, nil
	case "ERR":
		return ERR, nil
	case "REST":
		return REST, nil
	case "DONE":
		return DONE, nil
	default:
		return INVALID, newParseError("unknown instruction", str)
	}
}

// UnmarshalCommand parses command data into the appropriate Command type
func UnmarshalCommand(data []byte) (Command, error) {
	if len(data) == 0 {
		return nil, newProtocolError("unmarshal command", "empty command data")
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() {
		return nil, newProtocolError("unmarshal command", "failed to read command line")
	}

	line := scanner.Text()
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil, newProtocolError("unmarshal command", "empty command")
	}

	tcpInstr, err := ParseTcpInstruction(parts[0])
	if err != nil {
		return nil, err // Already a ProtocolError from ParseTcpInstruction
	}

	var cmd Command
	switch tcpInstr {
	case GET:
		cmd = &GetCommand{}
	case RETR:
		cmd = &RetrCommand{}
	case OK:
		cmd = &OkCommand{}
	case ERR:
		cmd = &ErrCommand{}
	case REST:
		cmd = &RestCommand{}
	case DONE:
		cmd = &DoneCommand{}
	default:
		return nil, newProtocolError("unmarshal command", fmt.Sprintf("unknown command: %s", tcpInstr))
	}

	if err := cmd.UnmarshalBinary(data); err != nil {
		// Wrap unmarshaling errors in ProtocolError if they aren't already
		if _, ok := err.(*ProtocolError); ok {
			return nil, err
		}
		return nil, newProtocolError("unmarshal command", fmt.Sprintf("failed to unmarshal %s command: %v", tcpInstr, err))
	}

	return cmd, nil
}

// GetCommand represents a GET request for file transfer
type GetCommand struct {
	Filename  string
	Blocksize uint64
	UdpPort   uint64
}

func (c *GetCommand) Instruction() TcpInstruction {
	return GET
}

func (c *GetCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %s %d %d\n", GET, c.Filename, c.Blocksize, c.UdpPort)
	return b.Bytes(), nil
}

func (c *GetCommand) UnmarshalBinary(data []byte) error {
	line := strings.TrimSpace(string(data))
	parts := strings.Fields(line)
	if len(parts) != 4 {
		return newParseError("GET command format", fmt.Sprintf("expected 4 fields, got %d", len(parts)))
	}

	// Parse instruction
	parsedInstr, err := ParseTcpInstruction(parts[0])
	if err != nil {
		return err
	}
	if parsedInstr != GET {
		return newProtocolError("GET command validation", fmt.Sprintf("expected GET, got %s", parsedInstr))
	}

	// Parse filename (parts[1])
	filename := parts[1]

	// Parse blocksize
	blocksize, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return newParseError("GET command format", fmt.Sprintf("invalid blocksize '%s': %v", parts[2], err))
	}

	// Parse UDP port
	udpPort, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		return newParseError("GET command format", fmt.Sprintf("invalid UDP port '%s': %v", parts[3], err))
	}

	// Validate parameters
	if filename == "" {
		return newValidationError("GET command", "filename cannot be empty")
	}
	if blocksize == 0 {
		return newValidationError("GET command", "blocksize must be greater than 0")
	}
	if udpPort == 0 || udpPort > 65535 {
		return newValidationError("GET command", fmt.Sprintf("UDP port must be 1-65535, got %d", udpPort))
	}

	c.Filename = filename
	c.Blocksize = blocksize
	c.UdpPort = udpPort
	return nil
}

// OkCommand represents a successful response with file size
type OkCommand struct {
	Filesize uint64
}

func (c *OkCommand) Instruction() TcpInstruction {
	return OK
}

func (c *OkCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %d\n", OK, c.Filesize)
	return b.Bytes(), nil
}

func (c *OkCommand) UnmarshalBinary(data []byte) error {
	line := strings.TrimSpace(string(data))
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return newParseError("OK command format", fmt.Sprintf("expected 2 fields, got %d", len(parts)))
	}

	// Parse instruction
	parsedInstr, err := ParseTcpInstruction(parts[0])
	if err != nil {
		return err
	}
	if parsedInstr != OK {
		return newProtocolError("OK command validation", fmt.Sprintf("expected OK, got %s", parsedInstr))
	}

	// Parse filesize
	filesize, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return newParseError("OK command format", fmt.Sprintf("invalid filesize '%s': %v", parts[1], err))
	}

	c.Filesize = filesize
	return nil
}

// RetrCommand represents a request to retransmit a specific block
type RetrCommand struct {
	BlockIndex uint64
}

func (c *RetrCommand) Instruction() TcpInstruction {
	return RETR
}

func (c *RetrCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %d\n", RETR, c.BlockIndex)
	return b.Bytes(), nil
}

func (c *RetrCommand) UnmarshalBinary(data []byte) error {
	line := strings.TrimSpace(string(data))
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return newParseError("RETR command format", fmt.Sprintf("expected 2 fields, got %d", len(parts)))
	}

	// Parse instruction
	parsedInstr, err := ParseTcpInstruction(parts[0])
	if err != nil {
		return err
	}
	if parsedInstr != RETR {
		return newProtocolError("RETR command validation", fmt.Sprintf("expected RETR, got %s", parsedInstr))
	}

	// Parse block index
	blockIndex, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return newParseError("RETR command format", fmt.Sprintf("invalid block index '%s': %v", parts[1], err))
	}

	c.BlockIndex = blockIndex
	return nil
}

// RestCommand represents a request to restart transmission from a specific block
type RestCommand struct {
	BlockIndex uint64
}

func (c *RestCommand) Instruction() TcpInstruction {
	return REST
}

func (c *RestCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %d\n", REST, c.BlockIndex)
	return b.Bytes(), nil
}

func (c *RestCommand) UnmarshalBinary(data []byte) error {
	line := strings.TrimSpace(string(data))
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return newParseError("REST command format", fmt.Sprintf("expected 2 fields, got %d", len(parts)))
	}

	// Parse instruction
	parsedInstr, err := ParseTcpInstruction(parts[0])
	if err != nil {
		return err
	}
	if parsedInstr != REST {
		return newProtocolError("REST command validation", fmt.Sprintf("expected REST, got %s", parsedInstr))
	}

	// Parse block index
	blockIndex, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return newParseError("REST command format", fmt.Sprintf("invalid block index '%s': %v", parts[1], err))
	}

	c.BlockIndex = blockIndex
	return nil
}

// ErrCommand represents an error response
type ErrCommand struct {
	Msg string
}

func (c *ErrCommand) Instruction() TcpInstruction {
	return ERR
}

func (c *ErrCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %s\n", ERR, c.Msg)
	return b.Bytes(), nil
}

func (c *ErrCommand) UnmarshalBinary(data []byte) error {
	// ERR command format: "ERR message text here"
	// The message can contain spaces, so we handle it differently
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(strings.ToUpper(line), "ERR ") {
		return newProtocolError("ERR command validation", "command must start with ERR")
	}

	// Extract message after "ERR "
	if len(line) <= 4 {
		return newValidationError("ERR command", "error message cannot be empty")
	}

	c.Msg = strings.TrimSpace(line[4:]) // Remove "ERR " prefix
	return nil
}

// DoneCommand represents completion of file transfer
type DoneCommand struct{}

func (c *DoneCommand) Instruction() TcpInstruction {
	return DONE
}

func (c *DoneCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s\n", DONE)
	return b.Bytes(), nil
}

func (c *DoneCommand) UnmarshalBinary(data []byte) error {
	line := strings.TrimSpace(string(data))
	parts := strings.Fields(line)
	if len(parts) != 1 {
		return newParseError("DONE command format", fmt.Sprintf("expected 1 field, got %d", len(parts)))
	}

	// Parse instruction
	parsedInstr, err := ParseTcpInstruction(parts[0])
	if err != nil {
		return err
	}
	if parsedInstr != DONE {
		return newProtocolError("DONE command validation", fmt.Sprintf("expected DONE, got %s", parsedInstr))
	}

	return nil
}
