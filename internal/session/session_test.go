package session

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fixture.jsonl")
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestCodexToolsAndDedup(t *testing.T) {
	p := fixture(t, `{"type":"session_meta","payload":{"id":"abc","cwd":"/test"}}
{"type":"response_item","payload":{"type":"message","id":"m1","role":"user","content":[{"type":"input_text","text":"你好"}]}}
{"type":"response_item","payload":{"type":"message","id":"m1","role":"user","content":[{"type":"input_text","text":"你好"}]}}
{"type":"response_item","payload":{"type":"function_call","call_id":"c1","name":"shell","arguments":"{\"cmd\":\"test\"}"}}
{"type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"needle hidden in output"}}
{"type":"event_msg","payload":{"type":"thread_rolled_back"}}
{"type":"new-format","payload":{}}
invalid
`)
	s, err := (JSONLAdapter{Agent: "codex"}).Parse(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "abc" || len(s.Entries) != 2 || s.Entries[1].Output != "needle hidden in output" {
		t.Fatalf("unexpected session: %+v", s)
	}
	if len(s.Warnings) != 3 {
		t.Fatalf("warnings: %v", s.Warnings)
	}
	if strings.Contains(Compact(s), "needle hidden") {
		t.Fatal("tool output leaked into compact transcript")
	}
}
func TestClaudeToolAndContext(t *testing.T) {
	p := fixture(t, `{"type":"assistant","uuid":"a","sessionId":"abc","message":{"role":"assistant","content":[{"type":"text","text":"Working"},{"type":"tool_use","id":"t1","name":"Read","input":{"path":"x"}}]}}
{"type":"user","uuid":"b","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"failure"}],"is_error":true}]}}
{"type":"user","uuid":"c","isMeta":true,"message":{"content":"injected context"}}
`)
	s, err := (JSONLAdapter{Agent: "claude"}).Parse(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Entries) != 3 || !s.Entries[1].Failed || s.Entries[2].Role != "context" {
		t.Fatalf("entries: %+v", s.Entries)
	}
}
func TestUnicodePages(t *testing.T) {
	s := strings.Repeat("你好🙂\n", 100)
	pages := Pages(s, 7)
	if strings.Join(pages, "") != s {
		t.Fatal("pagination lost content")
	}
	for _, p := range pages {
		if len([]rune(p)) > 7 {
			t.Fatal("page oversized")
		}
	}
}
func TestResolveAmbiguous(t *testing.T) {
	ss := []*Session{{Agent: "codex", ID: "abc"}, {Agent: "claude", ID: "abc"}}
	if _, err := Resolve(ss, "abc"); err == nil {
		t.Fatal("ambiguous ID accepted")
	}
	if s, err := Resolve(ss, "claude:abc"); err != nil || s.Agent != "claude" {
		t.Fatal("qualified lookup failed")
	}
}
func TestLargeRecord(t *testing.T) {
	body := `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"` + strings.Repeat("x", 100000) + `"}]}}`
	s, err := (JSONLAdapter{Agent: "codex"}).Parse(fixture(t, body))
	if err != nil || len(s.Entries) != 1 || len(s.Entries[0].Text) != 100000 {
		t.Fatal("large JSONL record failed", err)
	}
}

func TestClaudeAttachment(t *testing.T) {
	p := fixture(t, `{"type":"attachment","attachment":{"type":"queued_command","prompt":"please continue"}}
{"type":"attachment","attachment":{"type":"date_change","newDate":"2026-10-07"}}
{"type":"continued-in","continuedInSessionId":"next-session"}
`)
	s, err := (JSONLAdapter{Agent: "claude"}).Parse(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Entries) != 1 || s.Entries[0].Text != "please continue" || s.Entries[0].Role != "context" {
		t.Fatalf("attachment lost: %+v", s.Entries)
	}
	if len(s.Warnings) != 1 {
		t.Fatal(s.Warnings)
	}
}

func TestCompactSummarizesContextWithoutChangingSource(t *testing.T) {
	text := strings.Repeat("runtime detail ", 100)
	s := &Session{Entries: []Entry{{Role: "context", Name: "skill_listing", Text: text}, {Role: "user", Text: "human request"}}}
	got := Compact(s)
	if strings.Contains(got, text) || !strings.Contains(got, "human request") || s.Entries[0].Text != text {
		t.Fatal("context compression lost original or changed human text")
	}
}

func TestCompactOptimization(t *testing.T) {
	reminder := "full runtime budget reminder"
	s := &Session{Entries: []Entry{
		{Role: "user", Text: "keep the human request"},
		{Role: "context", Name: "total_tokens_reminder", Text: reminder},
		{Role: "context", Name: "total_tokens_reminder", Text: reminder},
		{Role: "context", Name: "queued_command", Text: "queued human request"},
		{Role: "tool", ToolID: "very-long-tool-id-1", Name: "Bash", Input: `{"command":"go test ./...","description":"noise"}`, Output: "full output retained"},
	}}
	got := Compact(s)
	if strings.Contains(got, reminder) || strings.Contains(got, "noise") || strings.Contains(got, "output:") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "total_tokens_reminder ×2") || !strings.Contains(got, "queued human request") || !strings.Contains(got, "go test ./...") {
		t.Fatal(got)
	}
	if s.Entries[1].Text != reminder || s.Entries[4].Output != "full output retained" {
		t.Fatal("original data mutated")
	}
}
func TestAliasCollisionsAndStability(t *testing.T) {
	s := &Session{}
	for i := 0; i < 1000; i++ {
		s.Entries = append(s.Entries, Entry{Role: "tool", ToolID: fmt.Sprintf("tool-%d", i)})
	}
	aliases := ToolAliases(s)
	seen := map[string]bool{}
	extended := false
	for _, a := range aliases {
		if seen[a] {
			t.Fatal("duplicate alias")
		}
		seen[a] = true
		if len(a) > 4 {
			extended = true
		}
	}
	if !extended {
		t.Fatal("fixture did not exercise a short-hash collision")
	}
	if !reflect.DeepEqual(aliases, ToolAliases(s)) {
		t.Fatal("unstable aliases")
	}
}

func TestRejectTranscriptSymlink(t *testing.T) {
	target := fixture(t, `{"type":"session_meta","payload":{"id":"outside"}}`)
	root := t.TempDir()
	link := filepath.Join(root, "link.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	if _, err := (JSONLAdapter{Agent: "codex"}).Parse(link); err == nil {
		t.Fatal("symlink followed")
	}
	sessions, warnings := Scan([]Source{{Root: root, Adapter: JSONLAdapter{Agent: "codex"}}})
	if len(sessions) != 0 || len(warnings) != 1 {
		t.Fatalf("sessions=%d warnings=%v", len(sessions), warnings)
	}
}
