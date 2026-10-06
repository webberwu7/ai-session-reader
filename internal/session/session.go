// Package session normalizes provider transcripts without changing their source files.
package session

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Entry struct {
	Role   string
	Text   string
	ToolID string
	Name   string
	Input  string
	Output string
	Failed bool
}
type Session struct {
	Agent     string
	ID        string
	Path      string
	Project   string
	Timestamp string
	Entries   []Entry
	Warnings  []string
	RawBytes  int64
}

// Adapter is the extension point for new providers, including future Antigravity support.
type Adapter interface {
	Name() string
	Parse(string) (*Session, error)
}
type JSONLAdapter struct{ Agent string }

func (a JSONLAdapter) Name() string { return a.Agent }

type obj map[string]any

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}
func mapOf(v any) obj {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return obj{}
}
func arr(v any) []any { a, _ := v.([]any); return a }
func text(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	var parts []string
	for _, x := range arr(v) {
		m := mapOf(x)
		if t := str(m["text"]); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n")
}

func (a JSONLAdapter) Parse(path string) (*Session, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing non-regular transcript file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := &Session{Agent: a.Agent, Path: path, ID: strings.TrimSuffix(filepath.Base(path), ".jsonl")}
	if st, err := f.Stat(); err == nil {
		s.RawBytes = st.Size()
	}
	reader := bufio.NewReader(f)
	seen := map[string]bool{}
	tools := map[string]int{}
	line := 0
	addTool := func(id, name, input string) {
		if id == "" {
			id = fmt.Sprintf("line-%d", line)
		}
		if _, ok := tools[id]; ok {
			return
		}
		tools[id] = len(s.Entries)
		s.Entries = append(s.Entries, Entry{Role: "tool", ToolID: id, Name: name, Input: input})
	}
	addResult := func(id, output string, failed bool) {
		if i, ok := tools[id]; ok {
			s.Entries[i].Output = output
			s.Entries[i].Failed = failed
		} else {
			s.Warnings = append(s.Warnings, fmt.Sprintf("line %d: orphan tool result %s", line, id))
			s.Entries = append(s.Entries, Entry{Role: "tool", ToolID: id, Name: "result", Output: output, Failed: failed})
		}
	}
	for {
		raw, e := reader.ReadBytes('\n')
		if len(raw) > 0 {
			line++
			var d obj
			if err := json.Unmarshal(raw, &d); err != nil {
				s.Warnings = append(s.Warnings, fmt.Sprintf("line %d: invalid JSON", line))
			} else {
				typ := str(d["type"])
				if s.Timestamp == "" {
					s.Timestamp = str(d["timestamp"])
				}
				if a.Agent == "claude" {
					if id := str(d["sessionId"]); id != "" {
						s.ID = id
					}
					if cwd := str(d["cwd"]); cwd != "" {
						s.Project = cwd
					}
					uuid := str(d["uuid"])
					duplicate := uuid != "" && seen[uuid]
					if uuid != "" {
						seen[uuid] = true
					}
					if !duplicate {
						switch typ {
						case "user", "assistant":
							msg := mapOf(d["message"])
							role := str(msg["role"])
							if role == "" {
								role = typ
							}
							if d["isMeta"] == true {
								role = "context"
							}
							content := msg["content"]
							if t, ok := content.(string); ok {
								s.Entries = append(s.Entries, Entry{Role: role, Text: t})
							} else {
								for _, x := range arr(content) {
									b := mapOf(x)
									switch str(b["type"]) {
									case "text":
										s.Entries = append(s.Entries, Entry{Role: role, Text: str(b["text"])})
									case "tool_use":
										addTool(str(b["id"]), str(b["name"]), str(b["input"]))
									case "tool_result":
										out := text(b["content"])
										if out == "" {
											out = str(b["content"])
										}
										addResult(str(b["tool_use_id"]), out, b["is_error"] == true)
									case "thinking", "redacted_thinking":
									default:
										s.Warnings = append(s.Warnings, fmt.Sprintf("line %d: unsupported Claude content %s", line, str(b["type"])))
									}
								}
							}
						case "summary":
							s.Entries = append(s.Entries, Entry{Role: "summary", Text: str(d["summary"])})
						case "attachment":
							attachment := mapOf(d["attachment"])
							var parts []string
							for _, key := range []string{"text", "content", "prompt", "context", "snippet"} {
								if value := str(attachment[key]); value != "" {
									parts = append(parts, value)
								}
							}
							if len(parts) > 0 {
								s.Entries = append(s.Entries, Entry{Role: "context", Name: str(attachment["type"]), Text: strings.Join(parts, "\n")})
							}
						case "continued-in":
							s.Warnings = append(s.Warnings, "session continues in another transcript: "+str(d["continuedInSessionId"]))
						case "system", "file-history-snapshot", "queue-operation", "progress", "mode", "cost-state", "last-prompt", "saved_hook_context", "ai-title", "atis-latch", "permission-mode", "agent-name", "file-history-delta", "custom-title", "bridge-session":
						default:
							s.Warnings = append(s.Warnings, fmt.Sprintf("line %d: unsupported Claude record %s", line, typ))
						}
					}
				} else {
					p := mapOf(d["payload"])
					switch typ {
					case "session_meta":
						if id := str(p["id"]); id != "" {
							s.ID = id
						}
						s.Project = str(p["cwd"])
					case "response_item":
						id := str(p["id"])
						if id != "" && seen[id] {
							break
						}
						if id != "" {
							seen[id] = true
						}
						switch str(p["type"]) {
						case "message":
							role := str(p["role"])
							if role == "user" || role == "assistant" {
								s.Entries = append(s.Entries, Entry{Role: role, Text: text(p["content"])})
							}
						case "function_call", "custom_tool_call":
							input := str(p["arguments"])
							if input == "" {
								input = str(p["input"])
							}
							addTool(str(p["call_id"]), str(p["name"]), input)
						case "function_call_output", "custom_tool_call_output":
							addResult(str(p["call_id"]), str(p["output"]), false)
						case "reasoning", "compaction":
						default:
							s.Warnings = append(s.Warnings, fmt.Sprintf("line %d: unsupported Codex item %s", line, str(p["type"])))
						}
					case "compacted":
						s.Entries = append(s.Entries, Entry{Role: "summary", Text: str(p["message"])})
						s.Warnings = append(s.Warnings, "compacted history: output is a stored audit trail, not reconstructed active UI history")
					case "event_msg":
						if str(p["type"]) == "thread_rolled_back" {
							s.Warnings = append(s.Warnings, "rollback present: output includes stored audit history")
						}
					case "turn_context", "token_usage_record", "world_state":
					default:
						s.Warnings = append(s.Warnings, fmt.Sprintf("line %d: unsupported Codex record %s", line, typ))
					}
				}
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
	}
	if len(s.Entries) == 0 {
		s.Warnings = append(s.Warnings, "no supported conversation entries")
	}
	return s, nil
}

type Source struct {
	Root    string
	Adapter Adapter
}

func Scan(sources []Source) ([]*Session, []string) {
	var sessions []*Session
	var warnings []string
	for _, source := range sources {
		err := filepath.WalkDir(source.Root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".jsonl") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				warnings = append(warnings, path+": "+err.Error())
				return nil
			}
			if !info.Mode().IsRegular() {
				warnings = append(warnings, path+": skipped non-regular transcript file")
				return nil
			}
			s, err := source.Adapter.Parse(path)
			if err != nil {
				warnings = append(warnings, path+": "+err.Error())
				return nil
			}
			sessions = append(sessions, s)
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			warnings = append(warnings, source.Root+": "+err.Error())
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Timestamp == sessions[j].Timestamp {
			return sessions[i].Path < sessions[j].Path
		}
		return sessions[i].Timestamp > sessions[j].Timestamp
	})
	return sessions, warnings
}
func Resolve(sessions []*Session, id string) (*Session, error) {
	var found []*Session
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, id) || strings.HasPrefix(s.Agent+":"+s.ID, id) {
			found = append(found, s)
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("session %q not found", id)
	}
	if len(found) > 1 {
		return nil, fmt.Errorf("session %q is ambiguous (%d files); use an agent-qualified full ID or narrower roots", id, len(found))
	}
	return found[0], nil
}
func Short(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return string(r)
}

// ToolAliases creates deterministic, unique display IDs without changing source IDs.
func ToolAliases(s *Session) map[string]string {
	hashes := map[string]string{}
	for _, e := range s.Entries {
		if e.Role == "tool" {
			hashes[e.ToolID] = fmt.Sprintf("%x", sha256.Sum256([]byte(e.ToolID)))
		}
	}
	aliases := map[string]string{}
	for id, hash := range hashes {
		n := 4
		for n < len(hash) {
			collision := false
			for other, h := range hashes {
				if other != id && h[:n] == hash[:n] {
					collision = true
					break
				}
			}
			if !collision {
				break
			}
			n++
		}
		aliases[id] = hash[:n]
	}
	return aliases
}

// ToolSummary extracts useful arguments rather than printing JSON syntax.
func ToolSummary(e Entry) string {
	var input obj
	if json.Unmarshal([]byte(e.Input), &input) != nil {
		return Short(e.Input, 100)
	}
	name := strings.ToLower(e.Name)
	var keys []string
	switch {
	case strings.Contains(name, "bash") || strings.Contains(name, "exec_command") || name == "shell":
		keys = []string{"command", "cmd", "workdir"}
	case strings.Contains(name, "read") || strings.Contains(name, "write") || strings.Contains(name, "edit") || strings.Contains(name, "file"):
		keys = []string{"file_path", "path", "filename", "offset", "limit"}
	case strings.Contains(name, "search") || strings.Contains(name, "grep") || strings.Contains(name, "glob"):
		keys = []string{"query", "pattern", "path", "include"}
	case strings.Contains(name, "agent") || strings.Contains(name, "task"):
		keys = []string{"description", "task_name", "subagent_type"}
	default:
		keys = []string{"url", "selector", "target", "text", "query", "path", "command", "cmd"}
	}
	var parts []string
	for _, key := range keys {
		if v := str(input[key]); v != "" {
			parts = append(parts, v)
		}
	}
	if len(parts) == 0 {
		return Short(e.Input, 100)
	}
	return Short(strings.Join(parts, " | "), 100)
}

func Compact(s *Session) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Session: %s:%s\nProject: %s\nSource: %s\nMode: stored audit transcript\n", s.Agent, s.ID, s.Project, s.Path)
	for _, w := range s.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", w)
	}
	aliases := ToolAliases(s)
	seenContext := map[string]bool{}
	omitted := map[string]int{}
	for _, e := range s.Entries {
		if e.Role == "tool" {
			status := ""
			if e.Failed {
				status = " FAILED"
			}
			fmt.Fprintf(&b, "[Tool#%s] %s%s %s\n", aliases[e.ToolID], e.Name, status, ToolSummary(e))
		} else if e.Role == "context" {
			switch e.Name {
			case "total_tokens_reminder", "silent_turn_reminder", "skill_listing", "model":
				omitted[e.Name]++
				continue
			}
			key := e.Name + "\x00" + e.Text
			if seenContext[key] {
				omitted["duplicates"]++
				continue
			}
			seenContext[key] = true
			fmt.Fprintf(&b, "[Context %s] %s\n", e.Name, Short(e.Text, 100))
		} else {
			fmt.Fprintf(&b, "\n%s:\n%s\n", e.Role, e.Text)
		}
	}
	var keys []string
	for key := range omitted {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "[Context omitted: %s ×%d; original text remains searchable]\n", key, omitted[key])
	}
	return b.String()
}

// Pages limits Unicode characters, including splitting a single oversized message.
func Pages(s string, size int) []string {
	r := []rune(s)
	var pages []string
	for len(r) > 0 {
		n := min(size, len(r))
		pages = append(pages, string(r[:n]))
		r = r[n:]
	}
	if len(pages) == 0 {
		return []string{""}
	}
	return pages
}
