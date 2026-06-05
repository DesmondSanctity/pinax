package extractor_test

import (
	"strings"
	"testing"

	"pinax/internal/extractor"
)

func TestFromMarkdown_PrependsSource(t *testing.T) {
	got := extractor.FromMarkdown("https://example.com/x", "# Title\n\nBody")
	if !strings.HasPrefix(got, "Source: https://example.com/x") {
		t.Errorf("missing Source prefix: %q", got)
	}
	if !strings.Contains(got, "# Title") {
		t.Error("missing markdown body")
	}
}

func TestFromHTML_StripsNavAndScript(t *testing.T) {
	html := `<html><head><title>Doc</title></head><body>
        <nav>nav text</nav>
        <script>alert(1)</script>
        <main><h1>Hello</h1><p>World</p></main>
        <footer>foot</footer>
    </body></html>`
	out, err := extractor.FromHTML("https://e.com/p", html)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"nav text", "alert(1)", "foot"} {
		if strings.Contains(out, banned) {
			t.Errorf("output should not contain %q: %s", banned, out)
		}
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "World") {
		t.Errorf("missing main content: %s", out)
	}
}

func TestFromHTML_StripsNoiseClasses(t *testing.T) {
	html := `<html><body><main>
        <div class="sidebar">side nav</div>
        <div class="on-this-page">toc</div>
        <p>Real content</p>
    </main></body></html>`
	out, err := extractor.FromHTML("https://e.com/p", html)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "side nav") || strings.Contains(out, "toc") {
		t.Errorf("noise classes not stripped: %s", out)
	}
	if !strings.Contains(out, "Real content") {
		t.Errorf("real content missing: %s", out)
	}
}

func TestFromHTML_PrefersArticleOverBody(t *testing.T) {
	html := `<html><body>
        <p>boilerplate</p>
        <article><h1>T</h1><p>article body</p></article>
    </body></html>`
	out, err := extractor.FromHTML("https://e.com/p", html)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "article body") {
		t.Errorf("expected article content: %s", out)
	}
}

func TestFromHTML_CodeBlock(t *testing.T) {
	html := `<html><body><main>
        <pre><code class="language-go">func main() {}</code></pre>
    </main></body></html>`
	out, err := extractor.FromHTML("https://e.com/p", html)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "```go") || !strings.Contains(out, "func main()") {
		t.Errorf("expected fenced code block with language: %s", out)
	}
}

func TestFromHTML_Headings(t *testing.T) {
	html := `<html><body><main>
        <h1>One</h1><h2>Two</h2><h3>Three</h3>
    </main></body></html>`
	out, _ := extractor.FromHTML("u", html)
	for _, want := range []string{"# One", "## Two", "### Three"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing heading %q: %s", want, out)
		}
	}
}

func TestFromHTML_Lists(t *testing.T) {
	html := `<html><body><main><ul><li>a</li><li>b</li></ul></main></body></html>`
	out, _ := extractor.FromHTML("u", html)
	if !strings.Contains(out, "- a") || !strings.Contains(out, "- b") {
		t.Errorf("expected list items: %s", out)
	}
}

func TestFromHTML_SourceLine(t *testing.T) {
	out, _ := extractor.FromHTML("https://e.com/p", `<html><body><main><p>x</p></main></body></html>`)
	if !strings.Contains(out, "Source: https://e.com/p") {
		t.Errorf("missing source line: %s", out)
	}
}
