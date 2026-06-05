// Package extractor turns HTML or Markdown documentation pages into plain text
// suitable for an LLM context window. Navigation, footers, scripts and other
// non-content noise are stripped.
package extractor

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// FromMarkdown wraps raw Markdown with a Source line, leaving the body intact.
func FromMarkdown(url, markdown string) string {
	return fmt.Sprintf("Source: %s\n\n%s", url, strings.TrimSpace(markdown))
}

// FromHTML extracts readable Markdown-flavoured text from an HTML document.
// It strips nav/header/footer/aside and similar non-content elements, prefers
// <main>/<article>, and emits headings, paragraphs, lists, code blocks, and
// tables in a stable order.
func FromHTML(url, htmlBody string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		return "", err
	}

	stripNoise(doc)

	title := pickTitle(doc, url)
	main := pickMain(doc)
	if main == nil {
		return fmt.Sprintf("# %s\nSource: %s\n\n(Content could not be extracted)", title, url), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\nSource: %s\n", title, url)
	walk(&b, main, 0)
	return collapseBlankLines(b.String()), nil
}

var noiseTags = map[string]struct{}{
	"nav": {}, "header": {}, "footer": {}, "aside": {},
	"script": {}, "style": {}, "noscript": {}, "template": {},
}

var noiseClassRe = regexp.MustCompile(`(?i)sidebar|navbar|menu|cookie|banner|toc|breadcrumb|edit-this-page|on-this-page`)

func stripNoise(n *html.Node) {
	for child := n.FirstChild; child != nil; {
		next := child.NextSibling
		if shouldStrip(child) {
			n.RemoveChild(child)
		} else {
			stripNoise(child)
		}
		child = next
	}
}

func shouldStrip(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if _, ok := noiseTags[n.Data]; ok {
		return true
	}
	for _, a := range n.Attr {
		if (a.Key == "class" || a.Key == "id" || a.Key == "aria-label") && noiseClassRe.MatchString(a.Val) {
			return true
		}
	}
	return false
}

func pickTitle(doc *html.Node, fallback string) string {
	if h1 := findFirst(doc, "h1"); h1 != nil {
		if t := strings.TrimSpace(textOf(h1)); t != "" {
			return t
		}
	}
	if title := findFirst(doc, "title"); title != nil {
		if t := strings.TrimSpace(textOf(title)); t != "" {
			return t
		}
	}
	return fallback
}

func pickMain(doc *html.Node) *html.Node {
	for _, tag := range []string{"main", "article"} {
		if n := findFirst(doc, tag); n != nil {
			return n
		}
	}
	if body := findFirst(doc, "body"); body != nil {
		return body
	}
	return doc
}

func findFirst(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirst(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func textOf(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textOf(c))
	}
	return b.String()
}

func walk(b *strings.Builder, n *html.Node, depth int) {
	if n.Type == html.TextNode {
		t := strings.TrimSpace(n.Data)
		if t != "" {
			b.WriteString(t)
			b.WriteByte(' ')
		}
		return
	}
	if n.Type != html.ElementNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(b, c, depth)
		}
		return
	}

	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := int(n.Data[1] - '0')
		fmt.Fprintf(b, "\n\n%s %s\n", strings.Repeat("#", level), strings.TrimSpace(textOf(n)))

	case atom.P:
		t := strings.TrimSpace(textOf(n))
		if t != "" {
			b.WriteString("\n\n")
			b.WriteString(t)
		}

	case atom.Pre:
		code := findFirst(n, "code")
		lang := ""
		var text string
		if code != nil {
			lang = codeLanguage(code)
			text = textOf(code)
		} else {
			text = textOf(n)
		}
		fmt.Fprintf(b, "\n\n```%s\n%s\n```", lang, strings.TrimRight(text, "\n"))

	case atom.Code:
		// Skip — already handled by <pre>. Inline <code> is rendered as plain.
		t := strings.TrimSpace(textOf(n))
		if t != "" {
			fmt.Fprintf(b, "`%s`", t)
		}

	case atom.Ul, atom.Ol:
		b.WriteByte('\n')
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.DataAtom == atom.Li {
				fmt.Fprintf(b, "\n- %s", strings.TrimSpace(textOf(c)))
			}
		}

	case atom.Blockquote:
		t := strings.TrimSpace(textOf(n))
		if t != "" {
			fmt.Fprintf(b, "\n\n> %s", t)
		}

	case atom.Table:
		emitTable(b, n)

	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(b, c, depth+1)
		}
	}
}

func codeLanguage(code *html.Node) string {
	for _, a := range code.Attr {
		if a.Key != "class" {
			continue
		}
		for _, tok := range strings.Fields(a.Val) {
			if strings.HasPrefix(tok, "language-") {
				return strings.TrimPrefix(tok, "language-")
			}
		}
	}
	return ""
}

func emitTable(b *strings.Builder, table *html.Node) {
	var rows [][]string
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Tr {
			var row []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
					row = append(row, strings.TrimSpace(textOf(c)))
				}
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(table)
	if len(rows) == 0 {
		return
	}
	b.WriteString("\n\n| ")
	b.WriteString(strings.Join(rows[0], " | "))
	b.WriteString(" |\n| ")
	for i := range rows[0] {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString("---")
	}
	b.WriteString(" |")
	for _, row := range rows[1:] {
		b.WriteString("\n| ")
		b.WriteString(strings.Join(row, " | "))
		b.WriteString(" |")
	}
}

var blankLineRe = regexp.MustCompile(`\n{3,}`)

func collapseBlankLines(s string) string {
	return strings.TrimSpace(blankLineRe.ReplaceAllString(s, "\n\n"))
}
