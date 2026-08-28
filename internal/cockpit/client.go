package cockpit

import "context"

type Client interface {
	Protocol(context.Context) (ProtocolInfo, error)
	ListAccounts(context.Context) ([]Account, error)
	ListInstances(context.Context) ([]Instance, error)
	CreateInstance(context.Context, CreateInstanceSpec) (Instance, error)
	UpdateInstance(context.Context, string, InstancePatch) (Instance, error)
	StartInstance(context.Context, string) (Instance, error)
	StopInstance(context.Context, string) (Instance, error)
	FocusInstance(context.Context, string) error
	BindAccount(context.Context, string, string) (Instance, error)
}
