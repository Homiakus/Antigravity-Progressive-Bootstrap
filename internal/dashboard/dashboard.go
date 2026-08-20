package dashboard

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/capability"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/planner"
	"github.com/homiakus/agctl/internal/replan"
	"github.com/homiakus/agctl/internal/securityaudit"
	"github.com/homiakus/agctl/internal/tasks"
	"github.com/homiakus/agctl/internal/telemetry"
)

type Snapshot struct {
	GeneratedAt      string               `json:"generatedAt"`
	TaskCounts       map[string]int       `json:"taskCounts"`
	PlanStatusCounts map[string]int       `json:"planStatusCounts"`
	PlanCount        int                  `json:"planCount"`
	PlanRevisions    int                  `json:"planRevisions"`
	DynamicNodes     int                  `json:"dynamicNodes"`
	ReplanInbox      int                  `json:"replanInbox"`
	ReplanConfig     model.ReplanConfig   `json:"replanConfig"`
	CapabilityCounts map[string]int       `json:"capabilityCounts"`
	Security         model.SecurityReport `json:"security"`
	Telemetry        telemetry.Summary    `json:"telemetry"`
	RecentEvents     []telemetry.Event    `json:"recentEvents"`
	RecentTasks      []model.TaskRecord   `json:"recentTasks"`
}

func BuildSnapshot(p paths.Paths, workspace string) Snapshot {
	x := Snapshot{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), TaskCounts: map[string]int{}, PlanStatusCounts: map[string]int{}, CapabilityCounts: map[string]int{}}
	if ts, err := tasks.List(p); err == nil {
		for _, t := range ts {
			x.TaskCounts[t.Status]++
		}
		if len(ts) > 20 {
			ts = ts[:20]
		}
		x.RecentTasks = ts
	}
	if ps, err := planner.List(p); err == nil {
		x.PlanCount = len(ps)
		for _, pl := range ps {
			x.PlanRevisions += pl.Revision
			x.DynamicNodes += pl.DynamicNodeCount
			status := pl.Status
			if status == "" {
				status = "legacy"
			}
			x.PlanStatusCounts[status]++
		}
	}
	if cfg, err := replan.LoadConfig(p); err == nil {
		x.ReplanConfig = cfg
	}
	if inbox, err := replan.Inbox(p); err == nil {
		x.ReplanInbox = len(inbox)
	}
	if reg, err := capability.Load(p); err == nil {
		for _, c := range reg.Capabilities {
			if c.Enabled {
				x.CapabilityCounts[c.Kind]++
			}
		}
	}
	if sec, err := securityaudit.Audit(p, workspace); err == nil {
		x.Security = sec
	}
	if ev, err := telemetry.Recent(p, 200); err == nil {
		x.Telemetry = telemetry.Summarize(ev)
		if len(ev) > 30 {
			ev = ev[:30]
		}
		x.RecentEvents = ev
	}
	return x
}

func Serve(p paths.Paths, workspace, listen string, allowRemote bool) error {
	if strings.TrimSpace(listen) == "" {
		listen = "127.0.0.1:8787"
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	if !allowRemote && !isLoopbackHost(host) {
		return fmt.Errorf("refusing non-loopback dashboard bind %q without --allow-remote", listen)
	}
	server := &http.Server{Addr: listen, Handler: newHandler(p, workspace, listen), ReadHeaderTimeout: 5 * time.Second}
	fmt.Printf("agctl dashboard: http://%s\n", listen)
	return server.ListenAndServe()
}

func newHandler(p paths.Paths, workspace, listen string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, BuildSnapshot(p, workspace)) })
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) { ev, _ := telemetry.Recent(p, 200); writeJSON(w, ev) })
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) { xs, _ := tasks.List(p); writeJSON(w, xs) })
	mux.HandleFunc("/api/plans", func(w http.ResponseWriter, r *http.Request) { xs, _ := planner.List(p); writeJSON(w, xs) })
	mux.HandleFunc("/api/replan", func(w http.ResponseWriter, r *http.Request) { x, _ := replan.Status(p, ""); writeJSON(w, x) })
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(prometheus(BuildSnapshot(p, workspace))))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Execute(w, map[string]string{"Listen": listen})
	})
	return securityHeaders(mux)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
func isLoopbackHost(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func prometheus(s Snapshot) string {
	var b strings.Builder
	keys := make([]string, 0, len(s.TaskCounts))
	for k := range s.TaskCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "agctl_tasks{status=%q} %d\n", k, s.TaskCounts[k])
	}
	keys = keys[:0]
	for k := range s.PlanStatusCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "agctl_plans{status=%q} %d\n", k, s.PlanStatusCounts[k])
	}
	keys = keys[:0]
	for k := range s.CapabilityCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "agctl_capabilities{kind=%q} %d\n", k, s.CapabilityCounts[k])
	}
	fmt.Fprintf(&b, "agctl_plans_total %d\n", s.PlanCount)
	fmt.Fprintf(&b, "agctl_plan_revisions_total %d\n", s.PlanRevisions)
	fmt.Fprintf(&b, "agctl_dynamic_nodes_total %d\n", s.DynamicNodes)
	fmt.Fprintf(&b, "agctl_replan_inbox %d\n", s.ReplanInbox)
	fmt.Fprintf(&b, "agctl_security_score %d\n", s.Security.Score)
	fmt.Fprintf(&b, "agctl_telemetry_events_total %d\n", s.Telemetry.Total)
	return b.String()
}

var page = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>agctl Control Plane</title><style>
body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;margin:0;background:#0f1115;color:#e7eaf0}.wrap{max-width:1200px;margin:auto;padding:24px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:12px}.card{background:#171a21;border:1px solid #2a303b;border-radius:12px;padding:16px}h1,h2{margin-top:0}.big{font-size:32px;font-weight:700}.muted{color:#96a0b5}pre{white-space:pre-wrap;overflow-wrap:anywhere}.good{color:#72d572}.warn{color:#f0c674}.bad{color:#ff6b6b}table{width:100%;border-collapse:collapse}td,th{text-align:left;padding:8px;border-bottom:1px solid #2a303b}</style></head>
<body><div class="wrap"><h1>agctl 3.2.1 — Control Plane</h1><div class="muted">Автообновление каждые 3 секунды · {{.Listen}}</div><br>
<div class="grid"><div class="card"><div class="muted">Security</div><div id="security" class="big">—</div></div><div class="card"><div class="muted">Plans / revisions</div><div id="plans" class="big">—</div></div><div class="card"><div class="muted">Tasks</div><div id="tasks" class="big">—</div></div><div class="card"><div class="muted">Adaptive nodes / inbox</div><div id="adaptive" class="big">—</div></div><div class="card"><div class="muted">Capabilities</div><div id="caps" class="big">—</div></div></div><br>
<div class="grid"><div class="card"><h2>Task states</h2><pre id="taskstates">—</pre></div><div class="card"><h2>Capabilities</h2><pre id="capstates">—</pre></div><div class="card"><h2>Telemetry</h2><pre id="telemetry">—</pre></div></div><br>
<div class="card"><h2>Recent tasks</h2><table><thead><tr><th>Status</th><th>Node</th><th>Agent</th><th>Workspace</th></tr></thead><tbody id="taskrows"></tbody></table></div></div>
<script>
async function tick(){try{const r=await fetch('/api/snapshot',{cache:'no-store'});const s=await r.json();
const sec=document.getElementById('security');sec.textContent=(s.security.grade||'?')+' '+(s.security.score??0);sec.className='big '+((s.security.score||0)>=80?'good':(s.security.score||0)>=60?'warn':'bad');
document.getElementById('plans').textContent=(s.planCount||0)+' / '+(s.planRevisions||0);document.getElementById('adaptive').textContent=(s.dynamicNodes||0)+' / '+(s.replanInbox||0);document.getElementById('tasks').textContent=Object.values(s.taskCounts||{}).reduce((a,b)=>a+b,0);document.getElementById('caps').textContent=Object.values(s.capabilityCounts||{}).reduce((a,b)=>a+b,0);
document.getElementById('taskstates').textContent=JSON.stringify(s.taskCounts||{},null,2);document.getElementById('capstates').textContent=JSON.stringify(s.capabilityCounts||{},null,2);document.getElementById('telemetry').textContent=JSON.stringify(s.telemetry||{},null,2);
const rows=document.getElementById('taskrows');rows.innerHTML='';for(const t of (s.recentTasks||[])){const tr=document.createElement('tr');for(const v of [t.status,t.nodeId||'',t.agent||'',t.workspace||'']){const td=document.createElement('td');td.textContent=v;tr.appendChild(td)}rows.appendChild(tr)}}catch(e){console.error(e)}}tick();setInterval(tick,3000);
</script></body></html>`))
