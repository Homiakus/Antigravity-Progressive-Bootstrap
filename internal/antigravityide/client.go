package antigravityide

import "context"

type Client interface {
	Health(context.Context) (Health, error)
	Capabilities(context.Context) (Capabilities, error)
	Context(context.Context) (Context, error)
	ListConversations(context.Context) ([]Conversation, error)
	CreateConversation(context.Context) (Conversation, error)
	FocusConversation(context.Context, string) error
	SendMessage(context.Context, string, string) error
	OpenWorkspace(context.Context, string) (OpenWorkspaceResult, error)
}
