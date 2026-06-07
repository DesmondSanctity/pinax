package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// BFSCrawl performs a same-origin breadth-first crawl from startURL.
// It deduplicates URLs by their canonical form (lowercased host, no fragment,
// no trailing slash) and treats redirect targets as already-visited so a
// URL and its redirect destination are not both processed.
func BFSCrawl(ctx context.Context, startURL string, opts Options) ([]Page, error) {
	origin, err := siteOrigin(startURL)
	if err != nil {
		return nil, err
	}
	baseHost, err := url.Parse(origin)
	if err != nil {
		return nil, err
	}

	c := &bfs{
		opts:     opts,
		origin:   origin,
		baseHost: strings.ToLower(baseHost.Host),
		visited:  make(map[string]bool),
		domainTs: make(map[string]time.Time),
	}
	return c.run(ctx, startURL)
}

type bfs struct {
	opts     Options
	origin   string
	baseHost string

	mu       sync.Mutex
	visited  map[string]bool
	domainTs map[string]time.Time
	results  []Page
}

func (c *bfs) run(ctx context.Context, startURL string) ([]Page, error) {
	concurrency := c.opts.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	queue := make(chan string, 1024)
	var pending sync.WaitGroup

	enqueue := func(u string) {
		canonical := CanonicalURL(u)
		c.mu.Lock()
		if c.visited[canonical] || len(c.results) >= c.opts.MaxPages {
			c.mu.Unlock()
			return
		}
		c.visited[canonical] = true
		c.mu.Unlock()
		pending.Add(1)
		queue <- u
	}

	enqueue(startURL)

	var workers sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for u := range queue {
				func() {
					defer pending.Done()
					if ctx.Err() != nil {
						return
					}
					c.mu.Lock()
					if len(c.results) >= c.opts.MaxPages {
						c.mu.Unlock()
						return
					}
					c.mu.Unlock()

					c.enforceDelay(u)
					page, links, err := c.fetch(ctx, u)
					if err != nil {
						return
					}

					c.mu.Lock()
					if len(c.results) < c.opts.MaxPages {
						c.results = append(c.results, *page)
					}
					c.mu.Unlock()

					for _, link := range links {
						if !c.isSameHost(link) || IsExcluded(link) {
							continue
						}
						enqueue(link)
					}
				}()
			}
		}()
	}

	pending.Wait()
	close(queue)
	workers.Wait()

	c.mu.Lock()
	out := c.results
	c.mu.Unlock()
	return out, nil
}

func (c *bfs) enforceDelay(pageURL string) {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return
	}
	domain := parsed.Host
	c.mu.Lock()
	last := c.domainTs[domain]
	c.mu.Unlock()
	if wait := c.opts.Delay - time.Since(last); wait > 0 {
		time.Sleep(wait)
	}
	c.mu.Lock()
	c.domainTs[domain] = time.Now()
	c.mu.Unlock()
}

func (c *bfs) fetch(ctx context.Context, pageURL string) (*Page, []string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "text/markdown, text/html;q=0.9, */*;q=0.8")

	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			target := CanonicalURL(r.URL.String())
			c.mu.Lock()
			alreadyVisited := c.visited[target]
			c.visited[target] = true
			c.mu.Unlock()
			if alreadyVisited {
				// Skip following: the canonical target is already queued or
				// processed elsewhere. Returning ErrUseLastResponse hands the
				// 3xx response back so we treat this fetch as a no-op.
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetch %s: HTTP %d", pageURL, resp.StatusCode)
	}

	finalURL := resp.Request.URL.String()
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	title := extractTitle(doc)
	links := extractLinks(doc, resp.Request.URL)

	return &Page{
		URL:     finalURL,
		Title:   title,
		Section: ExtractSection(finalURL, c.origin),
	}, links, nil
}

func (c *bfs) isSameHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.ToLower(u.Host) == c.baseHost
}

func extractTitle(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
		return strings.TrimSpace(n.FirstChild.Data)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if t := extractTitle(child); t != "" {
			return t
		}
	}
	return ""
}

func extractLinks(n *html.Node, base *url.URL) []string {
	var out []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" {
					if resolved := resolveURL(base, attr.Val); resolved != "" {
						out = append(out, resolved)
					}
					break
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return out
}

func resolveURL(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "javascript:") {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	abs := base.ResolveReference(ref)
	abs.Fragment = ""
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return ""
	}
	return abs.String()
}
