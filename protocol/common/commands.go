package common

import (
	"bytes"
	"fmt"
	"strings"
)

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
		return INVALID, &ParseInstructionError{str}
	}
}

type Command interface {
	MarshalBinary() (data []byte, err error)
	UnmarshalBinary(data []byte) error
}

func UnmarshalCommand(data []byte) (Command, error) {
	b := bytes.NewBuffer(data)
	var instr string
	_, err := fmt.Fscan(b, &instr)
	if err != nil {
		return nil, err
	}
	tcpInstr, err := ParseTcpInstruction(instr)
	if err != nil {
		return nil, err
	}
	var ret Command
	switch tcpInstr {
	case GET:
		ret = &GetCommand{}
	default:
		return nil, fmt.Errorf("unknown command: %s", tcpInstr)
	}
	err = ret.UnmarshalBinary(data)
	return ret, err
}

type GetCommand struct {
	Filename  string
	Blocksize uint64
	UdpPort   uint64
}

func (c GetCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, GET, c.Filename, c.Blocksize, c.UdpPort)
	return b.Bytes(), nil
}
func (c *GetCommand) UnmarshalBinary(data []byte) error {
	var instr []byte
	var filename string
	var blocksize uint64
	var udpPort uint64
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &instr, &filename, &blocksize, &udpPort)
	if err != nil {
		return err
	}
	parsedInstr, err := ParseTcpInstruction(string(instr))
	if err != nil {
		return err
	}
	if parsedInstr != GET {
		return fmt.Errorf("invalid command: expected GET, received %s", parsedInstr)
	}
	c.Filename = filename
	c.Blocksize = blocksize
	c.UdpPort = udpPort
	return nil
}

type OkCommand struct {
	Filesize uint64
}

func (c OkCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, OK, c.Filesize)
	return b.Bytes(), nil
}
func (c *OkCommand) UnmarshalBinary(data []byte) error {
	var instr []byte
	var filesize uint64
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &instr, &filesize)
	if err != nil {
		return err
	}
	parsedInstr, err := ParseTcpInstruction(string(instr))
	if err != nil {
		return err
	}
	if parsedInstr != OK {
		return fmt.Errorf("invalid command: expected OK, recieved %s", parsedInstr)
	}
	c.Filesize = filesize
	return nil
}

type RetrCommand struct {
	BlockIndex uint64
}

func (c RetrCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, RETR, c.BlockIndex)
	return b.Bytes(), nil
}
func (c *RetrCommand) UnmarshalBinary(data []byte) error {
	var instr []byte
	var blockIndex uint64
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &instr, &blockIndex)
	if err != nil {
		return err
	}
	parsedInstr, err := ParseTcpInstruction(string(instr))
	if err != nil {
		return err
	}
	if parsedInstr != RETR {
		return fmt.Errorf("invalid command: expected RETR, recieved %s", parsedInstr)
	}
	c.BlockIndex = blockIndex
	return nil
}

type RestCommand struct {
	BlockIndex uint64
}

func (c RestCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, REST, c.BlockIndex)
	return b.Bytes(), nil
}
func (c *RestCommand) UnmarshalBinary(data []byte) error {
	var instr []byte
	var blockIndex uint64
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &instr, &blockIndex)
	if err != nil {
		return err
	}
	parsedInstr, err := ParseTcpInstruction(string(instr))
	if err != nil {
		return err
	}
	if parsedInstr != REST {
		return fmt.Errorf("invalid command: expected REST, recieved %s", parsedInstr)
	}
	c.BlockIndex = blockIndex
	return nil
}

type ErrCommand struct {
	Msg string
}

func (c ErrCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, ERR, c.Msg)
	return b.Bytes(), nil
}
func (c *ErrCommand) UnmarshalBinary(data []byte) error {
	var instr []byte
	var msg string
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &instr, &msg)
	if err != nil {
		return err
	}
	parsedInstr, err := ParseTcpInstruction(string(instr))
	if err != nil {
		return err
	}
	if parsedInstr != ERR {
		return fmt.Errorf("invalid command: expected ERR, recieved %s", parsedInstr)
	}
	c.Msg = msg
	return nil
}

type DoneCommand struct {
}

func (c DoneCommand) MarshalBinary() (data []byte, err error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, DONE)
	return b.Bytes(), nil
}
func (c *DoneCommand) UnmarshalBinary(data []byte) error {
	var instr []byte
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &instr)
	if err != nil {
		return err
	}
	parsedInstr, err := ParseTcpInstruction(string(instr))
	if err != nil {
		return err
	}
	if parsedInstr != ERR {
		return fmt.Errorf("invalid command: expected DONE, recieved %s", parsedInstr)
	}
	return nil
}
