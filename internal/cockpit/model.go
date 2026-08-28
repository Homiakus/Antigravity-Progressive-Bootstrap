package cockpit

const ProtocolVersion = 1

type ProtocolInfo struct {
	ProtocolVersion int    `json:"protocolVersion"`
	Build           string `json:"build,omitempty"`
}

type Account struct {
	ID       string `json:"id"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
	Plan     string `json:"plan,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type Instance struct {
	ID              string  `json:"id"`
	Name            string  `json:"name,omitempty"`
	UserDataDir     string  `json:"userDataDir"`
	WorkingDir      string  `json:"workingDir,omitempty"`
	BindAccountID   *string `json:"bindAccountId,omitempty"`
	LastPID         *uint32 `json:"lastPid,omitempty"`
	Running         bool    `json:"running"`
	Initialized     bool    `json:"initialized"`
	IsDefault       bool    `json:"isDefault,omitempty"`
	FollowLocal     bool    `json:"followLocalAccount,omitempty"`
}

type CreateInstanceSpec struct {
	Name                 string
	UserDataDir          string
	WorkingDir           string
	ExtraArgs            string
	BindAccountID        string
	CopySourceInstanceID string
	InitMode             string
}

type InstancePatch struct {
	Name          *string
	WorkingDir    *string
	ExtraArgs     *string
	BindAccountID *string
	UnbindAccount bool
}
