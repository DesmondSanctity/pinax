// Command pinax turns any documentation site into a local MCP server.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"pinax/internal/cache"
	"pinax/internal/crawler"
	"pinax/internal/logger"
	"pinax/internal/manifest"
	mcpserver "pinax/internal/mcp/server"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	cmd, rest := os.Args[1], os.Args[2:]

	var err error
	switch cmd {
	case "add":
		err = cmdAdd(rest)
	case "list":
		err = cmdList()
	case "remove", "rm":
		err = cmdRemove(rest)
	case "refresh":
		err = cmdRefresh(rest)
	case "search":
		err = cmdSearch(rest)
	case "serve":
		err = cmdServe(rest)
	case "cache":
		err = cmdCache(rest)
	case "config":
		err = cmdConfig(rest)
	case "-v", "--version", "version":
		printVersion(os.Stdout)
	case "-h", "--help", "help":
		if len(rest) > 0 {
			printCommandHelp(os.Stdout, rest[0])
			return
		}
		usage(os.Stdout)
	default:
		usage(os.Stderr)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pinax:", err)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	if isTerminalWriter(w) {
		printBanner(w)
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, `pinax — turn any docs site into a local MCP server

Usage:
  pinax add <url> [--name NAME] [--exclude PATTERN ...] [--max-pages N]
  pinax list
  pinax remove <name>
  pinax refresh <name> [--rebuild-index]
  pinax search <name> <query>
  pinax serve <name> [--http] [--port N]
  pinax cache clear [--older-than DURATION]
  pinax config claude [--project]
  pinax help <command>
  pinax --version`)
}

// printCommandHelp prints the FlagSet usage for a single subcommand.
func printCommandHelp(w io.Writer, cmd string) {
	switch cmd {
	case "add":
		fs, _ := newAddFlags()
		fmt.Fprintln(w, "Usage: pinax add <url> [flags]")
		fs.SetOutput(w)
		fs.PrintDefaults()
	case "serve":
		fs, _ := newServeFlags()
		fmt.Fprintln(w, "Usage: pinax serve <name> [flags]")
		fs.SetOutput(w)
		fs.PrintDefaults()
	case "cache":
		fs := newCacheFlags()
		fmt.Fprintln(w, "Usage: pinax cache clear [flags]")
		fs.SetOutput(w)
		fs.PrintDefaults()
	case "list", "remove", "rm", "refresh", "config", "search":
		usage(w)
	default:
		fmt.Fprintf(w, "unknown command %q\n\n", cmd)
		usage(w)
	}
}

// ---------- add ----------

type addOpts struct {
	name        *string
	maxPages    *int
	concurrency *int
	excludes    multiString
}

func newAddFlags() (*flag.FlagSet, *addOpts) {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	o := &addOpts{
		name:        fs.String("name", "", "manifest name (defaults to derived from host)"),
		maxPages:    fs.Int("max-pages", 0, "override default max pages"),
		concurrency: fs.Int("concurrency", 0, "override default concurrency"),
	}
	fs.Var(&o.excludes, "exclude", "URL substring(s) to skip (repeatable)")
	return fs, o
}

func cmdAdd(args []string) error {
	fs, o := newAddFlags()
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("add: missing <url>")
	}
	rawURL := fs.Arg(0)

	if *o.name == "" {
		*o.name = deriveName(rawURL)
	}

	opts := crawler.DefaultOptions()
	if *o.maxPages > 0 {
		opts.MaxPages = *o.maxPages
	}
	if *o.concurrency > 0 {
		opts.Concurrency = *o.concurrency
	}
	opts.ExcludePaths = o.excludes

	fmt.Printf("crawling %s ...\n", rawURL)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	start := time.Now()
	res, err := crawler.Crawl(ctx, rawURL, opts)
	if err != nil {
		return fmt.Errorf("crawl: %w", err)
	}
	fmt.Printf("discovered %d pages via %s in %s\n", len(res.Pages), res.Source, time.Since(start).Truncate(time.Millisecond))

	m := manifest.FromCrawlResult(*o.name, res)
	if err := manifest.Save(m); err != nil {
		return err
	}
	p, _ := manifest.Path(*o.name)
	fmt.Printf("saved manifest %s → %s\n", *o.name, p)
	return nil
}

// ---------- list ----------

func cmdList() error {
	names, err := manifest.List()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Println("(no manifests — run 'pinax add <url>' to create one)")
		return nil
	}
	for _, name := range names {
		m, err := manifest.Load(name)
		if err != nil {
			fmt.Printf("%-30s  ERROR: %v\n", name, err)
			continue
		}
		fmt.Printf("%-30s  %4d pages  %s  (%s)\n",
			name, len(m.Pages), m.BaseURL, m.Source)
	}
	return nil
}

// ---------- remove ----------

func cmdRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("remove: missing <name>")
	}
	return manifest.Delete(args[0])
}

// ---------- refresh ----------

func cmdRefresh(args []string) error {
	fs := flag.NewFlagSet("refresh", flag.ContinueOnError)
	rebuildOnly := fs.Bool("rebuild-index", false, "only rebuild the BM25 index; skip re-crawling")
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("refresh: missing <name>")
	}
	name := fs.Arg(0)
	m, err := manifest.Load(name)
	if err != nil {
		return err
	}
	if *rebuildOnly {
		if err := manifest.SaveIndex(name, manifest.BuildIndex(m.Pages)); err != nil {
			return err
		}
		fmt.Printf("rebuilt search index for %s (%d pages)\n", name, len(m.Pages))
		return nil
	}
	fmt.Printf("re-crawling %s ...\n", m.BaseURL)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	res, err := crawler.Crawl(ctx, m.BaseURL, crawler.DefaultOptions())
	if err != nil {
		return err
	}
	updated := manifest.FromCrawlResult(name, res)
	if err := manifest.Save(updated); err != nil {
		return err
	}
	fmt.Printf("refreshed %s — %d pages\n", name, len(updated.Pages))
	return nil
}

// ---------- search ----------

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("limit", 10, "max results")
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("search: usage: pinax search <name> <query>")
	}
	name := fs.Arg(0)
	query := strings.Join(fs.Args()[1:], " ")

	m, err := manifest.Load(name)
	if err != nil {
		return err
	}
	idx, err := manifest.LoadIndex(name)
	if err != nil {
		if err == manifest.ErrIndexMissing {
			return fmt.Errorf("no search index for %q — run 'pinax refresh %s --rebuild-index'", name, name)
		}
		return err
	}
	hits := idx.Search(query, *limit)
	if len(hits) == 0 {
		fmt.Println("(no matches)")
		return nil
	}
	for _, h := range hits {
		if h.DocID < 0 || h.DocID >= len(m.Pages) {
			continue
		}
		p := m.Pages[h.DocID]
		fmt.Printf("%7.3f  %-40s  %s\n", h.Score, truncate(p.Title, 40), p.URL)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// ---------- serve ----------

type serveOpts struct {
	useHTTP *bool
	port    *int
}

func newServeFlags() (*flag.FlagSet, *serveOpts) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	o := &serveOpts{
		useHTTP: fs.Bool("http", false, "run HTTP transport instead of stdio"),
		port:    fs.Int("port", 8080, "HTTP port"),
	}
	return fs, o
}

func cmdServe(args []string) error {
	fs, o := newServeFlags()
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}

	// Unified mode: no positional arg → expose every manifest under one server.
	// Legacy mode: `pinax serve <name>` → single-manifest behaviour preserved.
	unified := fs.NArg() == 0

	var (
		manifests   map[string]*manifest.Manifest
		reload      func() (map[string]*manifest.Manifest, error)
		displayName string
		labelForUI  string // shown in startup hint/HTTP banner
		err         error
	)
	if unified {
		manifests, err = manifest.LoadAll()
		if err != nil {
			return err
		}
		if len(manifests) == 0 {
			return fmt.Errorf("no docs indexed — run `pinax add <url>` first")
		}
		reload = manifest.LoadAll
		displayName = "pinax"
		labelForUI = "pinax (unified)"
	} else {
		name := fs.Arg(0)
		m, err := manifest.Load(name)
		if err != nil {
			return err
		}
		manifests = map[string]*manifest.Manifest{name: m}
		displayName = "pinax/" + name
		labelForUI = name
	}

	pinaxDir, err := pinaxHome()
	if err != nil {
		return err
	}
	c, err := cache.Open(filepath.Join(pinaxDir, "cache.db"), cache.DefaultTTL)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	logStore, err := logger.Open(filepath.Join(pinaxDir, "calls.db"))
	if err != nil {
		return err
	}
	defer func() { _ = logStore.Close() }()

	mcpSrv := mcpserver.New(manifests, c, logStore, displayName, reload)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	stdinIsTTY := isTerminal(os.Stdin)
	if !*o.useHTTP && stdinIsTTY {
		printConfigHint(unified, labelForUI)
	}

	if *o.useHTTP {
		addr := fmt.Sprintf(":%d", *o.port)
		fmt.Fprintf(os.Stderr, "pinax serving %s on http://localhost%s (POST /mcp, log UI /)\n", labelForUI, addr)
		return mcpserver.ListenAndServeHTTP(ctx, mcpSrv, logStore, mcpserver.HTTPOptions{Addr: addr})
	}
	return mcpserver.ServeStdio(ctx, mcpSrv)
}

func printConfigHint(unified bool, name string) {
	exe, _ := os.Executable()
	if exe == "" {
		exe = "pinax"
	}
	if unified {
		fmt.Fprintf(os.Stderr, `
pinax is waiting on stdio (unified mode, all docs available via list_docs).
To use from Claude Desktop, add to your config:

  "mcpServers": {
    "pinax": { "command": "%s", "args": ["serve"] }
  }

Or run 'pinax serve --http' for HTTP mode with the log viewer.

`, exe)
		return
	}
	fmt.Fprintf(os.Stderr, `
pinax is waiting on stdio. To use it from Claude Desktop, add to your config:

  "mcpServers": {
    "%s": { "command": "%s", "args": ["serve", "%s"] }
  }

Or run 'pinax serve %s --http' for HTTP mode with the log viewer.

`, name, exe, name, name)
}

// ---------- cache ----------

func newCacheFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("cache clear", flag.ContinueOnError)
	fs.Duration("older-than", 0, "only clear entries older than this duration")
	return fs
}

func cmdCache(args []string) error {
	if len(args) < 1 || args[0] != "clear" {
		return fmt.Errorf("cache: only 'cache clear' is supported")
	}
	fs := newCacheFlags()
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	older, _ := time.ParseDuration(fs.Lookup("older-than").Value.String())
	dir, err := pinaxHome()
	if err != nil {
		return err
	}
	c, err := cache.Open(filepath.Join(dir, "cache.db"), cache.DefaultTTL)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	var n int64
	if older > 0 {
		n, err = c.ClearOlderThan(older)
	} else {
		n, err = c.Clear()
	}
	if err != nil {
		return err
	}
	fmt.Printf("cleared %d cache entries\n", n)
	return nil
}

// ---------- config ----------

func cmdConfig(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("config: missing target (try 'config claude')")
	}
	switch args[0] {
	case "claude":
		project := false
		split := false
		for _, a := range args[1:] {
			switch a {
			case "--project":
				project = true
			case "--split":
				split = true
			}
		}
		return configClaude(project, split)
	default:
		return fmt.Errorf("config: unknown target %q", args[0])
	}
}

func configClaude(project, split bool) error {
	names, err := manifest.List()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("no manifests — run 'pinax add <url>' first")
	}
	exe, _ := os.Executable()
	if exe == "" {
		exe = "pinax"
	}

	var sb strings.Builder
	sb.WriteString("{\n  \"mcpServers\": {\n")
	if split {
		// Legacy: one MCP server entry per manifest.
		for i, n := range names {
			fmt.Fprintf(&sb, "    %q: { \"command\": %q, \"args\": [\"serve\", %q] }", n, exe, n)
			if i < len(names)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
	} else {
		// Unified (default): one entry serving every indexed docs site.
		fmt.Fprintf(&sb, "    \"pinax\": { \"command\": %q, \"args\": [\"serve\"] }\n", exe)
	}
	sb.WriteString("  }\n}\n")

	if project {
		fmt.Println("# Place the following in ./.mcp.json")
	} else {
		fmt.Println("# Add the mcpServers block to ~/Library/Application Support/Claude/claude_desktop_config.json")
	}
	fmt.Print(sb.String())
	return nil
}

// ---------- helpers ----------

func pinaxHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".pinax")
	return dir, os.MkdirAll(dir, 0o700)
}

func deriveName(rawURL string) string {
	s := strings.TrimPrefix(rawURL, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, ".", "-")
	if s == "" {
		s = "server"
	}
	return s
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

type multiString []string

func (m *multiString) String() string     { return strings.Join(*m, ",") }
func (m *multiString) Set(v string) error { *m = append(*m, v); return nil }

// reorderArgs moves positional args after any flags so flag.Parse, which
// stops at the first non-flag token, sees every flag regardless of position.
// It consults fs to know which flags are boolean (and therefore take no value).
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		if strings.Contains(a, "=") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		f := fs.Lookup(name)
		if f == nil {
			continue // unknown flag — let flag.Parse report the error
		}
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}
