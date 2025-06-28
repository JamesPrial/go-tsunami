package common_test

import (
	"bytes"
	"encoding/gob"
	"testing"

	common "github.com/jamesprial/go-tsunami/protocol/common"
)

func TestGet(t *testing.T) {
	getTests := []struct {
		command common.GetCommand
		str     string
	}{
		{command: common.GetCommand{Filename: "foo", Blocksize: 100, UdpPort: 200}, str: "GET foo 100 200\n"},
		{command: common.GetCommand{Filename: "bar", Blocksize: 150, UdpPort: 100}, str: "GET bar 150 100\n"},
	}

	for _, test := range getTests {
		wantCmd := test.command
		var buf bytes.Buffer
		encoder := gob.NewEncoder(&buf)
		err := encoder.Encode(wantCmd)
		if err != nil {
			t.Errorf("unexpected error encoding: %v", err)
		}

		decoder := gob.NewDecoder(&buf)
		var gotCmd common.GetCommand
		err = decoder.Decode(&gotCmd)
		if err != nil {
			t.Errorf("unexpected error decoding: %v", err)
		}
		if wantCmd != gotCmd {
			t.Errorf("want %v got %v, buf: %s", wantCmd, gotCmd, buf.String())
		}
	}
}
