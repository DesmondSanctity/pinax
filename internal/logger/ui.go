package logger

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// API returns an http.Handler exposing GET /api/logs and DELETE /api/logs.
// Mount under whatever prefix you like.
func (s *Store) API() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleList(w, r)
		case http.MethodDelete:
			s.handleClear(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/servers", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.DistinctServers())
	})
	return mux
}

func (s *Store) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	writeJSON(w, http.StatusOK, s.List(ListParams{
		Limit:  limit,
		Server: strings.TrimSpace(q.Get("server")),
		Tool:   strings.TrimSpace(q.Get("tool")),
		Status: strings.TrimSpace(q.Get("status")),
		Docs:   strings.TrimSpace(q.Get("docs")),
	}))
}

func (s *Store) handleClear(w http.ResponseWriter, _ *http.Request) {
	n, err := s.Clear()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": n})
}

// UI returns the inline HTML log viewer. Single-file, no external assets.
func UI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(uiHTML))
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

const uiHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>pinax · tool calls</title>
<style>
  :root { color-scheme: dark; }
  body { margin: 0; font: 14px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; background: #0e0f12; color: #d8dadd; }
  header { padding: 12px 18px; background: #16181d; border-bottom: 1px solid #25282f; display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
  header h1 { font-size: 14px; margin: 0; color: #6da8ff; }
  select, input, button { background: #1c1f25; color: #d8dadd; border: 1px solid #2c3038; padding: 4px 8px; border-radius: 4px; font: inherit; }
  button { cursor: pointer; }
  button:hover { background: #262a32; }
  .ok { color: #6dd58c; }
  .error { color: #ff8a8a; }
  table { width: 100%; border-collapse: collapse; }
  th, td { padding: 6px 12px; border-bottom: 1px solid #1c1f25; text-align: left; vertical-align: top; }
  th { font-weight: 600; color: #8b8f97; background: #14161a; position: sticky; top: 0; }
  tr.row { cursor: pointer; }
  tr.row:hover { background: #161922; }
  tr.detail td { background: #0a0b0e; color: #b8bcc4; }
  pre { white-space: pre-wrap; word-break: break-word; margin: 4px 0; }
  .muted { color: #6c7079; }
</style>
</head>
<body>
<header>
  <h1>pinax tool calls</h1>
  <label>server <select id="f-server"><option value="">all</option></select></label>
  <label>status <select id="f-status"><option value="">all</option><option>ok</option><option>error</option></select></label>
  <label>tool <input id="f-tool" placeholder="any" size="14"></label>
  <label>docs <input id="f-docs" placeholder="any" size="14"></label>
  <button id="refresh">refresh</button>
  <button id="clear">clear all</button>
  <span class="muted" id="meta"></span>
</header>
<table>
  <thead><tr><th>time</th><th>server</th><th>docs</th><th>tool</th><th>status</th><th>ms</th><th>preview</th></tr></thead>
  <tbody id="rows"></tbody>
</table>
<script>
const fServer = document.getElementById('f-server');
const fStatus = document.getElementById('f-status');
const fTool   = document.getElementById('f-tool');
const fDocs   = document.getElementById('f-docs');
const rows    = document.getElementById('rows');
const meta    = document.getElementById('meta');
let expanded = new Set();
async function loadServers() {
  const r = await fetch('/api/servers');
  const list = await r.json() || [];
  const cur = fServer.value;
  fServer.innerHTML = '<option value="">all</option>' + list.map(s => '<option>'+s+'</option>').join('');
  fServer.value = cur;
}
function esc(s) { return String(s||'').replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c])); }
async function load() {
  const params = new URLSearchParams();
  if (fServer.value) params.set('server', fServer.value);
  if (fStatus.value) params.set('status', fStatus.value);
  if (fTool.value)   params.set('tool', fTool.value);
  if (fDocs.value)   params.set('docs', fDocs.value);
  const r = await fetch('/api/logs?' + params);
  const data = await r.json() || [];
  meta.textContent = data.length + ' entries';
  rows.innerHTML = data.map(e => {
    const time = new Date(e.calledAt).toLocaleTimeString();
    const cls  = e.status === 'ok' ? 'ok' : 'error';
    const main = '<tr class="row" data-id="'+e.id+'">'
       + '<td>'+time+'</td>'
       + '<td>'+esc(e.serverName)+'</td>'
       + '<td>'+esc(e.docs||'')+'</td>'
       + '<td>'+esc(e.toolName)+'</td>'
       + '<td class="'+cls+'">'+e.status+'</td>'
       + '<td>'+e.durationMs+'</td>'
       + '<td>'+esc(e.resultPreview)+'</td></tr>';
    if (!expanded.has(e.id)) return main;
    return main + '<tr class="detail"><td colspan="7">'
       + '<pre>arguments: '+esc(e.arguments)+'</pre>'
       + (e.error ? '<pre class="error">error: '+esc(e.error)+'</pre>' : '')
       + '<pre>result: '+esc(e.resultPreview)+'</pre></td></tr>';
  }).join('');
}
rows.addEventListener('click', ev => {
  const tr = ev.target.closest('tr.row');
  if (!tr) return;
  const id = tr.dataset.id;
  if (expanded.has(id)) expanded.delete(id); else expanded.add(id);
  load();
});
document.getElementById('refresh').onclick = load;
document.getElementById('clear').onclick = async () => {
  if (!confirm('Clear all log entries?')) return;
  await fetch('/api/logs', { method: 'DELETE' });
  expanded.clear();
  load();
};
for (const el of [fServer, fStatus, fTool, fDocs]) el.addEventListener('change', load);
fTool.addEventListener('input', load);
fDocs.addEventListener('input', load);
loadServers().then(load);
setInterval(() => { loadServers(); load(); }, 3000);
</script>
</body></html>
`
