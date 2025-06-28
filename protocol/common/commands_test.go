package common_test

import (
	"reflect"
	"testing"

	common "github.com/jamesprial/go-tsunami/protocol/common"
)

func TestParseTcpInstruction(t *testing.T) {
	tests := []struct {
		in      string
		want    common.TcpInstruction
		wantErr bool
	}{
		{"GET", common.GET, false},
		{" retr ", common.RETR, false},
		{"Ok", common.OK, false},
		{"err", common.ERR, false},
		{" done\n", common.DONE, false},
		{"bogus", common.INVALID, true},
	}
	for _, tt := range tests {
		got, err := common.ParseTcpInstruction(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseTcpInstruction(%q) expected error", tt.in)
			}
			if got != common.INVALID {
				t.Errorf("ParseTcpInstruction(%q) = %v, want INVALID", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTcpInstruction(%q) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseTcpInstruction(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestGetCommandMarshalUnmarshal(t *testing.T) {
	cases := []common.GetCommand{
		{Filename: "foo", Blocksize: 1, UdpPort: 2},
		{Filename: "bar", Blocksize: 100, UdpPort: 200},
	}
	for _, c := range cases {
		data, err := c.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		var got common.GetCommand
		if err := got.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary() error = %v", err)
		}
		if !reflect.DeepEqual(c, got) {
			t.Errorf("Marshal/Unmarshal mismatch. got=%v want=%v", got, c)
		}
	}
}

func TestUnmarshalCommand(t *testing.T) {
	data := []byte("GET foo 100 200\n")
	cmd, err := common.UnmarshalCommand(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := cmd.(*common.GetCommand)
	if !ok {
		t.Fatalf("expected *GetCommand, got %T", cmd)
	}
	want := &common.GetCommand{Filename: "foo", Blocksize: 100, UdpPort: 200}
	if *got != *want {
		t.Fatalf("want %v got %v", *want, *got)
	}
}

func TestGetCommandUnmarshalError(t *testing.T) {
	// wrong instruction should produce an error
	data := []byte("PUT file 10 20\n")
	var cmd common.GetCommand
	if err := cmd.UnmarshalBinary(data); err == nil {
		t.Error("expected error decoding invalid instruction")
	}
}

func TestUnmarshalCommandInvalid(t *testing.T) {
	// invalid instruction should return error
	data := []byte("BOGUS\n")
	if _, err := common.UnmarshalCommand(data); err == nil {
		t.Error("expected error for invalid instruction")
	}
}
