package web

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/telemetry"
)

// Broker handles Server-Sent Events (SSE) broadcasting to connected clients.
type Broker struct {
	clients    map[chan []byte]bool
	clientLock sync.Mutex
	history    []LogEvent
	histLock   sync.RWMutex
}

type LogEvent struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
}

var globalBroker = newBroker()

func newBroker() *Broker {
	return &Broker{
		clients: make(map[chan []byte]bool),
		history: make([]LogEvent, 0, 500),
	}
}

func (b *Broker) Broadcast(level, source, message string) {
	ev := LogEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     strings.ToUpper(level),
		Source:    source,
		Message:   message,
	}

	b.histLock.Lock()
	if len(b.history) >= 500 {
		b.history = b.history[1:]
	}
	b.history = append(b.history, ev)
	b.histLock.Unlock()

	data, err := json.Marshal(ev)
	if err != nil {
		return
	}

	payload := []byte(fmt.Sprintf("data: %s\n\n", string(data)))

	b.clientLock.Lock()
	defer b.clientLock.Unlock()
	for ch := range b.clients {
		select {
		case ch <- payload:
		default:
		}
	}
}

func (b *Broker) History() []LogEvent {
	b.histLock.RLock()
	defer b.histLock.RUnlock()
	out := make([]LogEvent, len(b.history))
	copy(out, b.history)
	return out
}

func (b *Broker) addClient(ch chan []byte) {
	b.clientLock.Lock()
	b.clients[ch] = true
	b.clientLock.Unlock()
}

func (b *Broker) removeClient(ch chan []byte) {
	b.clientLock.Lock()
	delete(b.clients, ch)
	close(ch)
	b.clientLock.Unlock()
}

// Log logs a message through the global broker and stdout
func Log(level, source, message string) {
	globalBroker.Broadcast(level, source, message)
}

// Serve starts the Web Control Plane server
func Serve(p paths.Paths, workspace, listen string, autoOpenBrowser bool, allowRemote bool) error {
	if strings.TrimSpace(listen) == "" {
		listen = "127.0.0.1:8787"
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}

	if !allowRemote && !isLoopbackHost(host) {
		return fmt.Errorf("refusing non-loopback bind %q without --allow-remote", listen)
	}

	// Test if port is already open or find available
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		// Fallback to random local port if default is occupied and host is 127.0.0.1
		if host == "127.0.0.1" || host == "localhost" {
			listener, err = net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return fmt.Errorf("failed to bind local port: %w", err)
			}
			listen = listener.Addr().String()
			_, port, _ = net.SplitHostPort(listen)
		} else {
			return fmt.Errorf("failed to bind %s: %w", listen, err)
		}
	}

	url := fmt.Sprintf("http://127.0.0.1:%s", port)
	if host != "127.0.0.1" && host != "" {
		url = fmt.Sprintf("http://%s", listen)
	}

	handler := newRouter(p, workspace, listen, url)
	server := &http.Server{
		Handler:           securityHeaders(handler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	Log("INFO", "system", fmt.Sprintf("agctl Web Control Plane running on %s", url))
	fmt.Printf("\n======================================================\n")
	fmt.Printf("🚀 agctl Web Control Plane: %s\n", url)
	fmt.Printf("======================================================\n\n")

	if autoOpenBrowser {
		go func() {
			time.Sleep(350 * time.Millisecond)
			_ = OpenBrowser(url)
		}()
	}

	// Stream initial telemetry events to log
	go func() {
		if ev, err := telemetry.Recent(p, 10); err == nil {
			for _, e := range ev {
				Log("EVENT", e.Type, fmt.Sprintf("[%s] %s", e.Tool, e.Reason))
			}
		}
	}()

	return server.Serve(listener)
}

func isLoopbackHost(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self' 'unsafe-inline' 'unsafe-eval' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
