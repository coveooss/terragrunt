package util

import (
	"bytes"
	"fmt"
	"testing"
)

func TestLogCatcher_Write(t *testing.T) {
	t.Parallel()
	type packet struct {
		sent, expect      string
		expectSize, delay int
	}
	tests := []struct {
		name       string
		afterClose string
		sequence   []packet
	}{
		{"Single", "Hello world\n", []packet{
			{"Hello world\n", "Hello world\n", 12, 0},
		}},
		{"Two batch", "Incomplete message\n", []packet{
			{"Incomplete ", "", 11, 0},
			{"message\n", "Incomplete message\n", 8, 0},
		}},
		{"Flush after close", "Not CR terminated", []packet{
			{"Not CR ", "", 7, 0},
			{"terminated", "", 10, 0},
		}},
		{"Test with critical", "That's all", []packet{
			{"This is a [critical] situation\n", "", 31, 0},
			{"That's all", "", 10, 0},
		}},
		{"Test with Error", "", []packet{
			{"    [ERROR] This is an error", "", 28, 0},
		}},
		{"Test with cut warning", "", []packet{
			{"This [war", "", 9, 0},
			{"nin", "", 3, 0},
			{"g] message is cut", "", 17, 0},
		}},
		{"Test with notice (including warning)", "", []packet{
			{"[NOTICE] This is a [Warning] message\n", "", 37, 0},
		}},
		{"Test with info and debug (including warning)", "data ...\n", []packet{
			{"[info] This is an important information", "", 39, 0},
			{"\ndata ...\n", "data ...\n", 10, 0},
			{"\t\t[debug]Useless trace", "data ...\n", 22, 0},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target bytes.Buffer
			catcher := &LogCatcher{Writer: &target, Logger: CreateLogger(tt.name)}
			for i, p := range tt.sequence {
				i++
				gotN, err := catcher.Write([]byte(p.sent))
				if err != nil {
					t.Errorf("LogCatcher.Write() #%d error = %v", i, err)
					return
				}
				if gotN != p.expectSize {
					t.Errorf("LogCatcher.Write() #%d = %v, want %v", i, gotN, p.expectSize)
				}
				if target.String() != p.expect {
					t.Errorf("LogCatcher.Write() #%d buffer = %q, want %q", i, target.String(), p.expect)
				}
				if i == len(tt.sequence) {
					catcher.Close()
					if target.String() != tt.afterClose {
						t.Errorf("LogCatcher.Write() final buffer = %q, want %q", target.String(), tt.afterClose)
					}
				}
			}
		})
	}
}

func TestLogCatcher_WriteWithError(t *testing.T) {
	tests := []struct {
		name            string
		text            string
		err             error
		wantResultCount int
		wantErr         bool
	}{
		{"No error", "", nil, 0, false},
		{"Count error", "Write something\n", nil, 0, true},
		{"Several errors", "First line\n[debug] Hello\nNo output\n", nil, 14, true},
		{"Disk full", "", fmt.Errorf("Disk is full"), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catcher := &LogCatcher{Writer: &buggyWriter{tt.err}, Logger: CreateLogger(tt.name)}
			gotResultCount, err := catcher.Write([]byte(tt.text))
			if (err != nil) != tt.wantErr {
				t.Errorf("LogCatcher.Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotResultCount != tt.wantResultCount {
				t.Errorf("LogCatcher.Write() = %v, want %v", gotResultCount, tt.wantResultCount)
			}
		})
	}
}

type buggyWriter struct{ err error }

func (bw *buggyWriter) Write(buffer []byte) (int, error) {
	if bw.err != nil {
		return 0, bw.err
	}
	fmt.Printf("Output (%q)\n", string(buffer))
	return 0, nil
}
