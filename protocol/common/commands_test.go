package common_test

import (
	"reflect"
	"testing"

	"github.com/jamesprial/go-tsunami/protocol/common"
)

func TestParseTcpInstruction(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    common.TcpInstruction
		wantErr bool
	}{
		{
			name:    "valid GET uppercase",
			input:   "GET",
			want:    common.GET,
			wantErr: false,
		},
		{
			name:    "valid RETR with whitespace",
			input:   " retr ",
			want:    common.RETR,
			wantErr: false,
		},
		{
			name:    "valid OK mixed case",
			input:   "Ok",
			want:    common.OK,
			wantErr: false,
		},
		{
			name:    "valid ERR lowercase",
			input:   "err",
			want:    common.ERR,
			wantErr: false,
		},
		{
			name:    "valid DONE with newline",
			input:   " done\n",
			want:    common.DONE,
			wantErr: false,
		},
		{
			name:    "valid REST",
			input:   "REST",
			want:    common.REST,
			wantErr: false,
		},
		{
			name:    "invalid instruction",
			input:   "bogus",
			want:    common.INVALID,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    common.INVALID,
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			want:    common.INVALID,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := common.ParseTcpInstruction(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseTcpInstruction(%q): expected error, got nil", tt.input)
				}
				if got != common.INVALID {
					t.Errorf("ParseTcpInstruction(%q): expected INVALID, got %v", tt.input, got)
				}

				// Test error type
				if !common.IsParseError(err) {
					t.Errorf("ParseTcpInstruction(%q): expected parse error, got %T: %v", tt.input, err, err)
				}
			} else {
				if err != nil {
					t.Errorf("ParseTcpInstruction(%q): unexpected error: %v", tt.input, err)
				}
				if got != tt.want {
					t.Errorf("ParseTcpInstruction(%q): expected %v, got %v", tt.input, tt.want, got)
				}
			}
		})
	}
}

func TestUnmarshalCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantType  string
		wantErr   bool
		errorType string
	}{
		{
			name:     "valid GET command",
			input:    []byte("GET foo 100 200\n"),
			wantType: "*common.GetCommand",
			wantErr:  false,
		},
		{
			name:     "valid OK command",
			input:    []byte("OK 1024\n"),
			wantType: "*common.OkCommand",
			wantErr:  false,
		},
		{
			name:     "valid RETR command",
			input:    []byte("RETR 5\n"),
			wantType: "*common.RetrCommand",
			wantErr:  false,
		},
		{
			name:     "valid REST command",
			input:    []byte("REST 10\n"),
			wantType: "*common.RestCommand",
			wantErr:  false,
		},
		{
			name:     "valid ERR command",
			input:    []byte("ERR File not found\n"),
			wantType: "*common.ErrCommand",
			wantErr:  false,
		},
		{
			name:     "valid DONE command",
			input:    []byte("DONE\n"),
			wantType: "*common.DoneCommand",
			wantErr:  false,
		},
		{
			name:      "empty data",
			input:     []byte(""),
			wantErr:   true,
			errorType: "protocol",
		},
		{
			name:      "invalid instruction",
			input:     []byte("BOGUS\n"),
			wantErr:   true,
			errorType: "parse",
		},
		{
			name:      "empty command",
			input:     []byte("   \n"),
			wantErr:   true,
			errorType: "protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := common.UnmarshalCommand(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UnmarshalCommand(%q): expected error, got nil", tt.input)
				}

				// Check error type
				switch tt.errorType {
				case "protocol":
					if !common.IsProtocolError(err) {
						t.Errorf("UnmarshalCommand(%q): expected protocol error, got %T: %v", tt.input, err, err)
					}
				case "parse":
					if !common.IsParseError(err) {
						t.Errorf("UnmarshalCommand(%q): expected parse error, got %T: %v", tt.input, err, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("UnmarshalCommand(%q): unexpected error: %v", tt.input, err)
				}

				cmdType := reflect.TypeOf(cmd).String()
				if cmdType != tt.wantType {
					t.Errorf("UnmarshalCommand(%q): expected type %s, got %s", tt.input, tt.wantType, cmdType)
				}
			}
		})
	}
}

func TestGetCommandValidation(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantErr   bool
		errorType string
	}{
		{
			name:    "valid command",
			input:   []byte("GET test.txt 1024 8080\n"),
			wantErr: false,
		},
		{
			name:      "empty filename",
			input:     []byte("GET  1024 8080\n"),
			wantErr:   true,
			errorType: "parse", // Will fail during scanf
		},
		{
			name:      "zero blocksize",
			input:     []byte("GET test.txt 0 8080\n"),
			wantErr:   true,
			errorType: "validation",
		},
		{
			name:      "invalid UDP port - zero",
			input:     []byte("GET test.txt 1024 0\n"),
			wantErr:   true,
			errorType: "validation",
		},
		{
			name:      "invalid UDP port - too high",
			input:     []byte("GET test.txt 1024 99999\n"),
			wantErr:   true,
			errorType: "validation",
		},
		{
			name:      "missing parameters",
			input:     []byte("GET test.txt 1024\n"),
			wantErr:   true,
			errorType: "parse",
		},
		{
			name:      "non-numeric blocksize",
			input:     []byte("GET test.txt abc 8080\n"),
			wantErr:   true,
			errorType: "parse",
		},
		{
			name:      "wrong instruction",
			input:     []byte("OK test.txt 1024 8080\n"),
			wantErr:   true,
			errorType: "protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd common.GetCommand
			err := cmd.UnmarshalBinary(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetCommand.UnmarshalBinary(%q): expected error, got nil", tt.input)
				}

				// Check specific error type
				switch tt.errorType {
				case "validation":
					if !common.IsValidationError(err) {
						t.Errorf("GetCommand.UnmarshalBinary(%q): expected validation error, got %T: %v", tt.input, err, err)
					}
				case "parse":
					if !common.IsParseError(err) {
						t.Errorf("GetCommand.UnmarshalBinary(%q): expected parse error, got %T: %v", tt.input, err, err)
					}
				case "protocol":
					if !common.IsProtocolError(err) {
						t.Errorf("GetCommand.UnmarshalBinary(%q): expected protocol error, got %T: %v", tt.input, err, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("GetCommand.UnmarshalBinary(%q): unexpected error: %v", tt.input, err)
				}
			}
		})
	}
}

func TestErrCommandWithSpaces(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectedMsg string
		wantErr     bool
	}{
		{
			name:        "simple error message",
			input:       []byte("ERR File not found\n"),
			expectedMsg: "File not found",
			wantErr:     false,
		},
		{
			name:        "error message with multiple spaces",
			input:       []byte("ERR Could not open file: permission denied\n"),
			expectedMsg: "Could not open file: permission denied",
			wantErr:     false,
		},
		{
			name:        "error message with trailing spaces",
			input:       []byte("ERR Network timeout   \n"),
			expectedMsg: "Network timeout",
			wantErr:     false,
		},
		{
			name:    "empty error message",
			input:   []byte("ERR\n"),
			wantErr: true,
		},
		{
			name:    "only spaces after ERR",
			input:   []byte("ERR   \n"),
			wantErr: true,
		},
		{
			name:    "wrong instruction",
			input:   []byte("OK File not found\n"),
			wantErr: true,
		},
		{
			name:    "missing ERR prefix",
			input:   []byte("File not found\n"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd common.ErrCommand
			err := cmd.UnmarshalBinary(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ErrCommand.UnmarshalBinary(%q): expected error, got nil", tt.input)
				}
				if common.IsValidationError(err) {
					// Expected for empty messages
				} else if common.IsProtocolError(err) {
					// Expected for wrong format
				} else {
					t.Errorf("ErrCommand.UnmarshalBinary(%q): expected validation or protocol error, got %T: %v", tt.input, err, err)
				}
			} else {
				if err != nil {
					t.Errorf("ErrCommand.UnmarshalBinary(%q): unexpected error: %v", tt.input, err)
				}
				if cmd.Msg != tt.expectedMsg {
					t.Errorf("ErrCommand.UnmarshalBinary(%q): expected message %q, got %q", tt.input, tt.expectedMsg, cmd.Msg)
				}
			}
		})
	}
}

func TestFilenameWithSpaces(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected common.GetCommand
		wantErr  bool
	}{
		{
			name:  "filename without spaces",
			input: []byte("GET test.txt 1024 8080\n"),
			expected: common.GetCommand{
				Filename:  "test.txt",
				Blocksize: 1024,
				UdpPort:   8080,
			},
			wantErr: false,
		},
		{
			name:  "filename with spaces now works",
			input: []byte("GET my file with spaces.txt 1024 8080\n"),
			expected: common.GetCommand{
				Filename:  "my file with spaces.txt",
				Blocksize: 1024,
				UdpPort:   8080,
			},
			wantErr: false,
		},
		{
			name:  "filename with underscores works",
			input: []byte("GET my_file.txt 1024 8080\n"),
			expected: common.GetCommand{
				Filename:  "my_file.txt",
				Blocksize: 1024,
				UdpPort:   8080,
			},
			wantErr: false,
		},
		{
			name:  "filename with dashes works",
			input: []byte("GET my-file.txt 1024 8080\n"),
			expected: common.GetCommand{
				Filename:  "my-file.txt",
				Blocksize: 1024,
				UdpPort:   8080,
			},
			wantErr: false,
		},
		{
			name:    "not enough fields",
			input:   []byte("GET file.txt\n"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd common.GetCommand
			err := cmd.UnmarshalBinary(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got nil", tt.input)
				}
				// Should be a parse error due to field count mismatch
				if !common.IsParseError(err) {
					t.Errorf("Expected parse error, got %T: %v", err, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if cmd.Filename != tt.expected.Filename {
					t.Errorf("Expected filename %q, got %q", tt.expected.Filename, cmd.Filename)
				}
				if cmd.Blocksize != tt.expected.Blocksize {
					t.Errorf("Expected blocksize %d, got %d", tt.expected.Blocksize, cmd.Blocksize)
				}
				if cmd.UdpPort != tt.expected.UdpPort {
					t.Errorf("Expected UDP port %d, got %d", tt.expected.UdpPort, cmd.UdpPort)
				}
			}
		})
	}
}

func TestNegativeNumbers(t *testing.T) {
	t.Run("parse error properties", func(t *testing.T) {
		_, err := common.ParseTcpInstruction("INVALID")
		if err == nil {
			t.Error("Expected error for invalid instruction, got nil")
		}

		if !common.IsParseError(err) {
			t.Errorf("Expected parse error, got %T: %v", err, err)
		}

		if protocolErr, ok := err.(*common.ProtocolError); ok {
			if protocolErr.Code() != common.ErrParseError {
				t.Errorf("Expected ErrParseError, got %v", protocolErr.Code())
			}
			if protocolErr.Operation() != "unknown instruction" {
				t.Errorf("Expected operation 'unknown instruction', got %q", protocolErr.Operation())
			}
		} else {
			t.Errorf("Expected ProtocolError type, got %T: %v", err, err)
		}
	})

	t.Run("validation error properties", func(t *testing.T) {
		var cmd common.GetCommand
		err := cmd.UnmarshalBinary([]byte("GET test.txt 0 8080\n"))
		if err == nil {
			t.Error("Expected error for zero blocksize, got nil")
		}

		if !common.IsValidationError(err) {
			t.Errorf("Expected validation error, got %T: %v", err, err)
		}

		if protocolErr, ok := err.(*common.ProtocolError); ok {
			if protocolErr.Code() != common.ErrValidationFailed {
				t.Errorf("Expected ErrValidationFailed, got %v", protocolErr.Code())
			}
		} else {
			t.Errorf("Expected ProtocolError type, got %T: %v", err, err)
		}
	})
}

// Keep existing marshal/unmarshal tests but enhance with better error messages
func TestGetCommandMarshalUnmarshal(t *testing.T) {
	cases := []common.GetCommand{
		{Filename: "foo", Blocksize: 1, UdpPort: 2},
		{Filename: "bar", Blocksize: 100, UdpPort: 200},
		{Filename: "test-file.txt", Blocksize: 32768, UdpPort: 8081},
	}
	for _, c := range cases {
		t.Run(c.Filename, func(t *testing.T) {
			data, err := c.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary() error = %v", err)
			}
			var got common.GetCommand
			if err := got.UnmarshalBinary(data); err != nil {
				t.Fatalf("UnmarshalBinary() error = %v", err)
			}
			if !reflect.DeepEqual(c, got) {
				t.Errorf("Marshal/Unmarshal mismatch: expected %+v, got %+v", c, got)
			}
		})
	}
}

func TestOkCommandMarshalUnmarshal(t *testing.T) {
	cases := []common.OkCommand{
		{Filesize: 0},
		{Filesize: 123},
		{Filesize: 456},
		{Filesize: 1048576}, // 1MB
	}
	for _, c := range cases {
		t.Run(string(rune(c.Filesize)), func(t *testing.T) {
			data, err := c.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary() error = %v", err)
			}
			var got common.OkCommand
			if err := got.UnmarshalBinary(data); err != nil {
				t.Fatalf("UnmarshalBinary() error = %v", err)
			}
			if !reflect.DeepEqual(c, got) {
				t.Errorf("Marshal/Unmarshal mismatch: expected %+v, got %+v", c, got)
			}
		})
	}
}

func TestRetrCommandMarshalUnmarshal(t *testing.T) {
	cases := []common.RetrCommand{{BlockIndex: 1}, {BlockIndex: 99}, {BlockIndex: 0}}
	for _, c := range cases {
		t.Run(string(rune(c.BlockIndex)), func(t *testing.T) {
			data, err := c.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary() error = %v", err)
			}
			var got common.RetrCommand
			if err := got.UnmarshalBinary(data); err != nil {
				t.Fatalf("UnmarshalBinary() error = %v", err)
			}
			if !reflect.DeepEqual(c, got) {
				t.Errorf("Marshal/Unmarshal mismatch: expected %+v, got %+v", c, got)
			}
		})
	}

	t.Run("invalid instruction error", func(t *testing.T) {
		data := []byte("GET 10\n")
		var cmd common.RetrCommand
		err := cmd.UnmarshalBinary(data)
		if err == nil {
			t.Error("Expected error decoding invalid instruction, got nil")
		}
		if !common.IsProtocolError(err) {
			t.Errorf("Expected protocol error, got %T: %v", err, err)
		}
	})
}

func TestRestCommandMarshalUnmarshal(t *testing.T) {
	cases := []common.RestCommand{{BlockIndex: 2}, {BlockIndex: 1000}, {BlockIndex: 0}}
	for _, c := range cases {
		t.Run(string(rune(c.BlockIndex)), func(t *testing.T) {
			data, err := c.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary() error = %v", err)
			}
			var got common.RestCommand
			if err := got.UnmarshalBinary(data); err != nil {
				t.Fatalf("UnmarshalBinary() error = %v", err)
			}
			if !reflect.DeepEqual(c, got) {
				t.Errorf("Marshal/Unmarshal mismatch: expected %+v, got %+v", c, got)
			}
		})
	}

	t.Run("invalid instruction error", func(t *testing.T) {
		data := []byte("RETR 1\n")
		var cmd common.RestCommand
		err := cmd.UnmarshalBinary(data)
		if err == nil {
			t.Error("Expected error decoding invalid instruction, got nil")
		}
		if !common.IsProtocolError(err) {
			t.Errorf("Expected protocol error, got %T: %v", err, err)
		}
	})
}

func TestErrCommandMarshalUnmarshal(t *testing.T) {
	cases := []common.ErrCommand{
		{Msg: "oops"},
		{Msg: "bad"},
		{Msg: "File not found"},
		{Msg: "Permission denied: access forbidden"},
	}
	for _, c := range cases {
		t.Run(c.Msg, func(t *testing.T) {
			data, err := c.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary() error = %v", err)
			}
			var got common.ErrCommand
			if err := got.UnmarshalBinary(data); err != nil {
				t.Fatalf("UnmarshalBinary() error = %v", err)
			}
			if !reflect.DeepEqual(c, got) {
				t.Errorf("Marshal/Unmarshal mismatch: expected %+v, got %+v", c, got)
			}
		})
	}

	t.Run("invalid instruction error", func(t *testing.T) {
		data := []byte("OK all good\n")
		var cmd common.ErrCommand
		err := cmd.UnmarshalBinary(data)
		if err == nil {
			t.Error("Expected error decoding invalid instruction, got nil")
		}
		if !common.IsProtocolError(err) {
			t.Errorf("Expected protocol error, got %T: %v", err, err)
		}
	})
}

func TestDoneCommandMarshalUnmarshal(t *testing.T) {
	var c common.DoneCommand
	data, err := c.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}
	var got common.DoneCommand
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary() error = %v", err)
	}
	if !reflect.DeepEqual(c, got) {
		t.Errorf("Marshal/Unmarshal mismatch: expected %+v, got %+v", c, got)
	}

	t.Run("invalid instruction error", func(t *testing.T) {
		bad := []byte("ERR\n")
		err := got.UnmarshalBinary(bad)
		if err == nil {
			t.Error("Expected error decoding invalid instruction, got nil")
		}
		if !common.IsProtocolError(err) {
			t.Errorf("Expected protocol error, got %T: %v", err, err)
		}
	})
}

// Benchmark tests for performance
func BenchmarkGetCommandMarshal(b *testing.B) {
	cmd := common.GetCommand{
		Filename:  "test.txt",
		Blocksize: 32768,
		UdpPort:   8081,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cmd.MarshalBinary()
		if err != nil {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkGetCommandUnmarshal(b *testing.B) {
	data := []byte("GET test.txt 32768 8081\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var cmd common.GetCommand
		err := cmd.UnmarshalBinary(data)
		if err != nil {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkUnmarshalCommand(b *testing.B) {
	data := []byte("GET test.txt 32768 8081\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := common.UnmarshalCommand(data)
		if err != nil {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}
