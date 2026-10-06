package cli

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/webberwu7/ai-session-reader/internal/session"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Run(args []string, out, errout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprintln(out, "ai-session <list|search|read|context|inherit|expand|stats|doctor> [arguments] [flags]\nFlags: --agent claude|codex, --project text, --claude-root path, --codex-root path, --state-dir path, --limit 20, --page-size 20000, --page N, --reset, --json\nArguments and flags may appear in either order. inherit repeats advance pages; --reset restarts.")
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	codex := os.Getenv("CODEX_HOME")
	if codex == "" {
		codex = filepath.Join(home, ".codex")
	}
	claude := os.Getenv("CLAUDE_CONFIG_DIR")
	if claude == "" {
		claude = filepath.Join(home, ".claude")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(errout)
	agent := fs.String("agent", "", "provider filter")
	project := fs.String("project", "", "project substring")
	cr := fs.String("claude-root", filepath.Join(claude, "projects"), "Claude transcripts root")
	xr := fs.String("codex-root", filepath.Join(codex, "sessions"), "Codex transcripts root")
	state := fs.String("state-dir", filepath.Join(home, ".ai-session"), "pagination state")
	limit := fs.Int("limit", 20, "list/search result limit, 0 means unlimited")
	size := fs.Int("page-size", 20000, "characters per page")
	page := fs.Int("page", 0, "explicit 1-based page")
	reset := fs.Bool("reset", false, "restart pagination")
	asJSON := fs.Bool("json", false, "JSON output for list, search, stats, doctor")
	// Go's flag package stops at the first positional argument; reorder known flags.
	var flags, pos []string
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			name := strings.TrimLeft(strings.SplitN(a, "=", 2)[0], "-")
			if name != "reset" && name != "json" && name != "h" && name != "help" && !strings.Contains(a, "=") {
				if i+1 >= len(args) {
					return fmt.Errorf("missing value for %s", a)
				}
				i++
				flags = append(flags, args[i])
			}
		} else {
			pos = append(pos, a)
		}
	}
	if err := fs.Parse(flags); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *agent != "" && *agent != "claude" && *agent != "codex" {
		return fmt.Errorf("unsupported agent %q", *agent)
	}
	if *limit < 0 || *size < 1 || *page < 0 {
		return errors.New("limit/page must be nonnegative; page-size must be positive")
	}
	cmd := args[0]
	want := map[string]int{"list": 0, "doctor": 0, "search": 1, "read": 1, "context": 1, "inherit": 1, "expand": 2, "stats": 1}
	n, ok := want[cmd]
	if !ok {
		return fmt.Errorf("unknown command %q", cmd)
	}
	if len(pos) != n {
		return fmt.Errorf("%s needs %d argument(s)", cmd, n)
	}
	if cmd == "search" && pos[0] == "" {
		return errors.New("search query must not be empty")
	}
	for _, option := range []struct{ name, root string }{{"claude-root", *cr}, {"codex-root", *xr}} {
		var checkErr error
		fs.Visit(func(f *flag.Flag) {
			if f.Name == option.name {
				info, err := os.Stat(option.root)
				if err != nil {
					checkErr = fmt.Errorf("%s: %w", option.name, err)
				} else if !info.IsDir() {
					checkErr = fmt.Errorf("%s must be a directory", option.name)
				}
			}
		})
		if checkErr != nil {
			return checkErr
		}
	}
	var sources []session.Source
	if *agent == "" || *agent == "claude" {
		sources = append(sources, session.Source{Root: *cr, Adapter: session.JSONLAdapter{Agent: "claude"}})
	}
	if *agent == "" || *agent == "codex" {
		sources = append(sources, session.Source{Root: *xr, Adapter: session.JSONLAdapter{Agent: "codex"}})
		if *xr == filepath.Join(codex, "sessions") {
			sources = append(sources, session.Source{Root: filepath.Join(codex, "archived_sessions"), Adapter: session.JSONLAdapter{Agent: "codex"}})
		}
	}
	all, warnings := session.Scan(sources)
	for _, w := range warnings {
		fmt.Fprintln(errout, "warning:", w)
	}
	var filtered []*session.Session
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Project), strings.ToLower(*project)) {
			filtered = append(filtered, s)
		}
	}
	emit := func(v any) error { return json.NewEncoder(out).Encode(v) }
	switch cmd {
	case "list":
		count := 0
		for _, s := range filtered {
			if *limit > 0 && count >= *limit {
				break
			}
			title := ""
			for _, e := range s.Entries {
				if e.Role == "user" && e.Text != "" {
					title = session.Short(e.Text, 80)
					break
				}
			}
			if *asJSON {
				if err := emit(map[string]any{"agent": s.Agent, "id": s.ID, "project": s.Project, "timestamp": s.Timestamp, "title": title, "path": s.Path, "warnings": len(s.Warnings)}); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(out, "%s:%s  %s  %s  %s\n", s.Agent, s.ID, s.Timestamp, s.Project, title)
			}
			count++
		}
		return nil
	case "search":
		query := strings.ToLower(pos[0])
		if query == "" {
			return errors.New("search query must not be empty")
		}
		count := 0
		for _, s := range filtered {
			for i, e := range s.Entries {
				for _, field := range []struct{ name, value string }{{"text", e.Text}, {"input", e.Input}, {"output", e.Output}} {
					lower := strings.ToLower(field.value)
					idx := strings.Index(lower, query)
					if idx < 0 {
						continue
					}
					snippet := session.Short(field.value, 180)
					r := []rune(field.value)
					at := len([]rune(lower[:idx]))
					start := max(0, at-40)
					if start < len(r) {
						snippet = session.Short(string(r[start:]), 180)
					}
					if *asJSON {
						if err := emit(map[string]any{"session": s.Agent + ":" + s.ID, "entry": i + 1, "field": field.name, "tool_id": e.ToolID, "snippet": snippet}); err != nil {
							return err
						}
					} else {
						fmt.Fprintf(out, "%s:%s #%d %s %s\n", s.Agent, s.ID, i+1, field.name, snippet)
					}
					count++
					if *limit > 0 && count >= *limit {
						return nil
					}
				}
			}
		}
		return nil
	case "doctor":
		for _, s := range filtered {
			if *asJSON {
				if err := emit(map[string]any{"session": s.Agent + ":" + s.ID, "path": s.Path, "warnings": s.Warnings}); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(out, "%s:%s entries=%d warnings=%d\n", s.Agent, s.ID, len(s.Entries), len(s.Warnings))
				for _, w := range s.Warnings {
					fmt.Fprintln(out, "  "+w)
				}
			}
		}
		if len(warnings) > 0 {
			return errors.New("some source files could not be scanned")
		}
		return nil
	}
	s, err := session.Resolve(filtered, pos[0])
	if err != nil {
		return err
	}
	compact := session.Compact(s)
	switch cmd {
	case "read", "context":
		_, err = io.WriteString(out, compact)
		return err
	case "expand":
		var matches []session.Entry
		aliases := session.ToolAliases(s)
		// Exact source IDs and exact display aliases take precedence over prefixes.
		for _, e := range s.Entries {
			if e.Role == "tool" && (e.ToolID == pos[1] || aliases[e.ToolID] == pos[1]) {
				matches = append(matches, e)
			}
		}
		if len(matches) == 0 {
			for _, e := range s.Entries {
				if e.Role == "tool" && strings.HasPrefix(e.ToolID, pos[1]) {
					matches = append(matches, e)
				}
			}
		}
		if len(matches) != 1 {
			return fmt.Errorf("tool ID must match exactly one call (matched %d)", len(matches))
		}
		e := matches[0]
		fmt.Fprintf(out, "[Tool#%s] %s\nInput:\n%s\nOutput:\n%s\n", e.ToolID, e.Name, e.Input, e.Output)
		return nil
	case "stats":
		reduction := 0.0
		if s.RawBytes > 0 {
			reduction = 100 * (1 - float64(len(compact))/float64(s.RawBytes))
		}
		v := map[string]any{"session": s.Agent + ":" + s.ID, "raw_bytes": s.RawBytes, "compact_bytes": len(compact), "byte_reduction_percent": reduction, "entries": len(s.Entries), "warnings": len(s.Warnings), "note": "byte reduction is not token usage or billing"}
		if *asJSON {
			return emit(v)
		}
		fmt.Fprintf(out, "%s:%s\nRaw: %d bytes; compact: %d bytes; reduction: %.1f%%\nEntries: %d; warnings: %d\nByte reduction is not token usage or billing.\n", s.Agent, s.ID, s.RawBytes, len(compact), reduction, len(s.Entries), len(s.Warnings))
		return nil
	case "inherit":
		return inherit(out, s, compact, *state, *size, *page, *reset)
	}
	return nil
}

type progress struct {
	Digest string
	Next   int
}

func inherit(out io.Writer, s *session.Session, content, dir string, size, requested int, reset bool) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(s.Agent+":"+s.ID+":"+s.Path)))
	path := filepath.Join(dir, key+".json")
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("pagination state locked or unavailable: %w", err)
	}
	lock.Close()
	defer os.Remove(path + ".lock")
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%s", size, content))))
	p := progress{Digest: digest, Next: 1}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !reset {
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("invalid pagination state: %w", err)
		}
		if p.Digest != digest {
			p = progress{Digest: digest, Next: 1}
			fmt.Fprintln(out, "[transcript or page-size changed; restarted]")
		}
	}
	pages := session.Pages(content, size)
	current := p.Next
	if requested > 0 {
		current = requested
	}
	if current < 1 {
		return errors.New("invalid page state")
	}
	if current > len(pages) {
		if requested > 0 {
			return fmt.Errorf("page out of range (1-%d)", len(pages))
		}
		fmt.Fprintln(out, "[inherit complete; use --reset to restart]")
		return nil
	}
	if _, err := fmt.Fprintf(out, "[page %d/%d]\n%s\n", current, len(pages), pages[current-1]); err != nil {
		return err
	}
	if current == len(pages) {
		fmt.Fprintln(out, "[inherit complete]")
	}
	p.Next = current + 1
	p.Digest = digest
	data, _ := json.Marshal(p)
	tmp, err := os.CreateTemp(dir, "progress-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
