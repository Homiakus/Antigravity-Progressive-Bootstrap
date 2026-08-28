package antigravityide

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// InstanceCommandGatePool owns exactly one focus/send critical section per IDE
// instance. Direct conversation dispatch bypasses the gate completely, so
// independent instances and capable Bridges remain concurrent.
type InstanceCommandGatePool struct {
	mu    sync.Mutex
	gates map[string]*sync.Mutex
}

func NewInstanceCommandGatePool() *InstanceCommandGatePool {
	return &InstanceCommandGatePool{gates: map[string]*sync.Mutex{}}
}

func (p *InstanceCommandGatePool) gate(instanceID string) (*sync.Mutex, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return nil, fmt.Errorf("instance id is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gates == nil {
		p.gates = map[string]*sync.Mutex{}
	}
	gate := p.gates[instanceID]
	if gate == nil {
		gate = &sync.Mutex{}
		p.gates[instanceID] = gate
	}
	return gate, nil
}

// Send dispatches one message to one conversation. If the Bridge advertises
// direct dispatch, the operation is fully concurrent. Otherwise the complete
// verify -> focus -> send sequence is atomic for that IDE instance.
func (p *InstanceCommandGatePool) Send(ctx context.Context, instanceID string, client Client, caps Capabilities, conversationID, text string) error {
	if client == nil {
		return fmt.Errorf("Antigravity IDE client is required")
	}
	conversationID = strings.TrimSpace(conversationID)
	text = strings.TrimSpace(text)
	if conversationID == "" || text == "" {
		return fmt.Errorf("conversation id and message text are required")
	}
	if caps.ConversationDirectSend {
		return client.SendMessage(ctx, conversationID, text)
	}
	if !caps.ConversationFocus || !caps.ConversationSend || !caps.ConversationList {
		return fmt.Errorf("Bridge cannot safely dispatch by focus+send")
	}
	gate, err := p.gate(instanceID)
	if err != nil {
		return err
	}
	gate.Lock()
	defer gate.Unlock()

	conversations, err := client.ListConversations(ctx)
	if err != nil {
		return fmt.Errorf("verify conversation before dispatch: %w", err)
	}
	found := false
	for _, conversation := range conversations {
		if conversation.ID == conversationID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("conversation %q is not present in instance %q", conversationID, instanceID)
	}
	if err := client.FocusConversation(ctx, conversationID); err != nil {
		return fmt.Errorf("focus conversation %q: %w", conversationID, err)
	}
	if err := client.SendMessage(ctx, conversationID, text); err != nil {
		return fmt.Errorf("send to conversation %q: %w", conversationID, err)
	}
	return nil
}
