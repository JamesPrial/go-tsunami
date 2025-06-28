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
	_, err := fmt.Fscanln(b, &instr)
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
		return fmt.Errorf("invalid command: expected GET, recieved %s", parsedInstr)
	}
	c.Filename = filename
	c.Blocksize = blocksize
	c.UdpPort = udpPort
	return err
}
