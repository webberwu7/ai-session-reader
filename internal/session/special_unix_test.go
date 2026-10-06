//go:build darwin || linux

package session

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestRejectTranscriptFIFO(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pipe.jsonl")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (JSONLAdapter{Agent: "codex"}).Parse(path); err == nil {
		t.Fatal("FIFO accepted")
	}
	sessions, warnings := Scan([]Source{{Root: root, Adapter: JSONLAdapter{Agent: "codex"}}})
	if len(sessions) != 0 || len(warnings) != 1 {
		t.Fatalf("sessions=%d warnings=%v", len(sessions), warnings)
	}
}
