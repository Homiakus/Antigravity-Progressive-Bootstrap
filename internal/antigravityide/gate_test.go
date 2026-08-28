package antigravityide

import (
	"context"
	"sync"
	"testing"
	"time"
)

type focusDependentClient struct {
	mu      sync.Mutex
	active  string
	routed  map[string][]string
	started chan struct{}
}

func newFocusDependentClient() *focusDependentClient {
	return &focusDependentClient{routed: map[string][]string{}, started: make(chan struct{}, 8)}
}

func (f *focusDependentClient) Health(context.Context) (Health, error) { return Health{Status: "ok"}, nil }
func (f *focusDependentClient) Capabilities(context.Context) (Capabilities, error) { return Capabilities{}, nil }
func (f *focusDependentClient) Context(context.Context) (Context, error) { return Context{}, nil }
func (f *focusDependentClient) ListConversations(context.Context) ([]Conversation, error) {
	return []Conversation{{ID: "A"}, {ID: "B"}}, nil
}
func (f *focusDependentClient) CreateConversation(context.Context) (Conversation, error) { return Conversation{}, nil }
func (f *focusDependentClient) FocusConversation(_ context.Context, id string) error {
	f.mu.Lock()
	f.active = id
	f.mu.Unlock()
	select { case f.started <- struct{}{}: default: }
	time.Sleep(10 * time.Millisecond)
	return nil
}
func (f *focusDependentClient) SendMessage(_ context.Context, _ string, text string) error {
	f.mu.Lock()
	active := f.active
	f.routed[active] = append(f.routed[active], text)
	f.mu.Unlock()
	return nil
}
func (f *focusDependentClient) OpenWorkspace(context.Context, string) (OpenWorkspaceResult, error) { return OpenWorkspaceResult{}, nil }

func TestInstanceCommandGateSerializesFocusAndSend(t *testing.T) {
	client := newFocusDependentClient()
	pool := NewInstanceCommandGatePool()
	caps := Capabilities{ConversationList: true, ConversationFocus: true, ConversationSend: true, ConversationDirectSend: false}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); if err := pool.Send(context.Background(), "instance-1", client, caps, "A", "message-A"); err != nil { t.Errorf("send A: %v", err) } }()
	go func() { defer wg.Done(); if err := pool.Send(context.Background(), "instance-1", client, caps, "B", "message-B"); err != nil { t.Errorf("send B: %v", err) } }()
	wg.Wait()
	client.mu.Lock()
	defer client.mu.Unlock()
	if got := client.routed["A"]; len(got) != 1 || got[0] != "message-A" { t.Fatalf("conversation A routed=%v", got) }
	if got := client.routed["B"]; len(got) != 1 || got[0] != "message-B" { t.Fatalf("conversation B routed=%v", got) }
}

type directClient struct {
	focusCalls int
	sendCalls  int
}
func (d *directClient) Health(context.Context) (Health, error) { return Health{Status: "ok"}, nil }
func (d *directClient) Capabilities(context.Context) (Capabilities, error) { return Capabilities{}, nil }
func (d *directClient) Context(context.Context) (Context, error) { return Context{}, nil }
func (d *directClient) ListConversations(context.Context) ([]Conversation, error) { return nil, nil }
func (d *directClient) CreateConversation(context.Context) (Conversation, error) { return Conversation{}, nil }
func (d *directClient) FocusConversation(context.Context, string) error { d.focusCalls++; return nil }
func (d *directClient) SendMessage(context.Context, string, string) error { d.sendCalls++; return nil }
func (d *directClient) OpenWorkspace(context.Context, string) (OpenWorkspaceResult, error) { return OpenWorkspaceResult{}, nil }

func TestInstanceCommandGateBypassesFocusForDirectDispatch(t *testing.T) {
	client := &directClient{}
	pool := NewInstanceCommandGatePool()
	caps := Capabilities{ConversationDirectSend: true}
	if err := pool.Send(context.Background(), "instance-1", client, caps, "A", "hello"); err != nil { t.Fatal(err) }
	if client.focusCalls != 0 || client.sendCalls != 1 { t.Fatalf("focus=%d send=%d", client.focusCalls, client.sendCalls) }
}
