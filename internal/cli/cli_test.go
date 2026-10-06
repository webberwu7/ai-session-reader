package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSearchAndPagination(t *testing.T) {
	root := t.TempDir()
	body := `{"type":"session_meta","payload":{"id":"test-id"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"你好🙂hello"}]}}
{"type":"response_item","payload":{"type":"custom_tool_call","call_id":"c","name":"exec","input":"run"}}
{"type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"c","output":"secret-needle"}}
`
	if err := os.WriteFile(filepath.Join(root, "rollout.jsonl"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"--agent", "codex", "--codex-root", root, "--state-dir", filepath.Join(root, "state"), "--page-size", "90"}
	var out, errout bytes.Buffer
	run := func(args ...string) string {
		t.Helper()
		out.Reset()
		if err := Run(append(args, base...), &out, &errout); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if got := run("search", "secret-needle"); !strings.Contains(got, "output secret-needle") {
		t.Fatal(got)
	}
	if got := run("read", "test-id"); strings.Contains(got, "secret-needle") {
		t.Fatal("compact read leaked output")
	}
	if got := run("expand", "test-id", "c"); !strings.Contains(got, "secret-needle") {
		t.Fatal(got)
	}
	compact := run("read", "test-id")
	alias := regexp.MustCompile(`\[Tool#([^\]]+)\]`).FindStringSubmatch(compact)[1]
	if got := run("expand", "test-id", alias); !strings.Contains(got, "secret-needle") {
		t.Fatal("display alias expansion failed", got)
	}
	first := run("inherit", "test-id")
	if !strings.Contains(first, "page 1/") {
		t.Fatal(first)
	}
	second := run("inherit", "test-id")
	if !strings.Contains(second, "page 2/") {
		t.Fatal(second)
	}
	if got := run("inherit", "test-id", "--reset"); got != first {
		t.Fatalf("reset mismatch: %q vs %q", got, first)
	}
	for i := 0; i < 20; i++ {
		if strings.Contains(run("inherit", "test-id"), "inherit complete") {
			break
		}
		if i == 19 {
			t.Fatal("never completed")
		}
	}
	if got := run("inherit", "test-id"); !strings.Contains(got, "use --reset") {
		t.Fatal(got)
	}
	if err := Run(append([]string{"inherit", "test-id", "--page", "999"}, base...), &out, &errout); err == nil {
		t.Fatal("invalid page accepted")
	}
}
func TestInvalidFlags(t *testing.T) {
	for _, args := range [][]string{{"read"}, {"search", ""}, {"list", "--agent", "antigravity"}, {"list", "--page-size", "0"}, {"other"}} {
		var b bytes.Buffer
		if err := Run(args, &b, &b); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
