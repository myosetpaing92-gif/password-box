package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantText string
	}{
		{
			name:     "help",
			args:     []string{"--help"},
			wantCode: 0,
			wantText: "Usage:",
		},
		{
			name:     "version",
			args:     []string{"version"},
			wantCode: 0,
			wantText: "pbox dev\n",
		},
		{
			name:     "unknown command",
			args:     []string{"unknown"},
			wantCode: 2,
			wantText: "Error:",
		},
		{
			name:     "unknown flag",
			args:     []string{"--unknown"},
			wantCode: 2,
			wantText: "Error:",
		},
		{
			name:     "unexpected version argument",
			args:     []string{"version", "extra"},
			wantCode: 2,
			wantText: "Error:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := Run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Fatalf("exit code = %d; want %d", code, tt.wantCode)
			}

			output := stdout.String()
			if tt.wantCode != 0 {
				output = stderr.String()

				if stdout.Len() != 0 {
					t.Errorf("unexpected stdout: %q", stdout.String())
				}
			} else if stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %q", stderr.String())
			}

			if !strings.Contains(output, tt.wantText) {
				t.Errorf("output = %q; want %q", output, tt.wantText)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("output unavailable")
}

func TestRunOutputFailure(t *testing.T) {
	var stderr bytes.Buffer

	code := Run([]string{"version"}, failingWriter{}, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d; want 1", code)
	}
}
