package runtime

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/cockpit"
)

var ErrBridgeCredentialsUnavailable = errors.New("remote runtime: live Bridge credentials unavailable")

type SecretSource interface {
	NewSecret(int) (string, error)
}

type CryptoSecretSource struct{}

func (CryptoSecretSource) NewSecret(bytes int) (string, error) {
	if bytes <= 0 {
		return "", fmt.Errorf("secret byte count must be positive")
	}
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate runtime secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

type Manager interface {
	Start(context.Context, cockpit.Instance) (cockpit.Instance, antigravityide.LocatedBridge, error)
	Stop(context.Context, string) (cockpit.Instance, error)
	Bridge(string) (antigravityide.LocatedBridge, bool)
	Forget(string)
}

type Options struct {
	Cockpit        cockpit.ManagedClient
	Locator        antigravityide.Locator
	Secrets        SecretSource
	BridgeRegistry string
}

type LiveManager struct {
	cockpit        cockpit.ManagedClient
	locator        antigravityide.Locator
	secrets        SecretSource
	bridgeRegistry string

	mu      sync.RWMutex
	bridges map[string]antigravityide.LocatedBridge
}

func New(opts Options) (*LiveManager, error) {
	if opts.Cockpit == nil || opts.Locator == nil {
		return nil, fmt.Errorf("remote runtime Cockpit client and Bridge locator are required")
	}
	if strings.TrimSpace(opts.BridgeRegistry) == "" {
		return nil, fmt.Errorf("remote runtime Bridge registry is required")
	}
	secrets := opts.Secrets
	if secrets == nil {
		secrets = CryptoSecretSource{}
	}
	return &LiveManager{cockpit: opts.Cockpit, locator: opts.Locator, secrets: secrets, bridgeRegistry: opts.BridgeRegistry, bridges: map[string]antigravityide.LocatedBridge{}}, nil
}

func (m *LiveManager) Start(ctx context.Context, instance cockpit.Instance) (cockpit.Instance, antigravityide.LocatedBridge, error) {
	if strings.TrimSpace(instance.ID) == "" {
		return cockpit.Instance{}, antigravityide.LocatedBridge{}, fmt.Errorf("runtime start requires instance id")
	}
	if instance.Running {
		if bridge, ok := m.Bridge(instance.ID); ok {
			health, err := bridge.Client.Health(ctx)
			if err == nil && (health.InstanceID == "" || health.InstanceID == instance.ID) {
				return instance, bridge, nil
			}
			m.Forget(instance.ID)
		}
		return cockpit.Instance{}, antigravityide.LocatedBridge{}, fmt.Errorf("instance %s is already running: %w", instance.ID, ErrBridgeCredentialsUnavailable)
	}

	bootNonce, err := m.secrets.NewSecret(16)
	if err != nil {
		return cockpit.Instance{}, antigravityide.LocatedBridge{}, err
	}
	bridgeToken, err := m.secrets.NewSecret(32)
	if err != nil {
		return cockpit.Instance{}, antigravityide.LocatedBridge{}, err
	}
	started, err := m.cockpit.StartManagedInstance(ctx, instance.ID, cockpit.LaunchContext{
		InstanceID:     instance.ID,
		BootNonce:      bootNonce,
		BridgeToken:    bridgeToken,
		BridgeRegistry: m.bridgeRegistry,
	})
	if err != nil {
		return cockpit.Instance{}, antigravityide.LocatedBridge{}, err
	}
	located, err := m.locator.Wait(ctx, instance.ID, bridgeToken)
	if err != nil {
		return started, antigravityide.LocatedBridge{}, err
	}
	health, err := located.Client.Health(ctx)
	if err != nil {
		return started, antigravityide.LocatedBridge{}, err
	}
	if health.InstanceID != "" && health.InstanceID != instance.ID {
		return started, antigravityide.LocatedBridge{}, fmt.Errorf("Bridge instance identity %q does not match %q", health.InstanceID, instance.ID)
	}
	if located.Registration.BootNonce != "" && located.Registration.BootNonce != bootNonce {
		return started, antigravityide.LocatedBridge{}, fmt.Errorf("Bridge boot nonce mismatch")
	}
	if started.LastPID != nil && located.Registration.PID > 0 && int(*started.LastPID) != located.Registration.PID {
		return started, antigravityide.LocatedBridge{}, fmt.Errorf("Bridge pid %d does not match Cockpit pid %d", located.Registration.PID, *started.LastPID)
	}
	m.remember(instance.ID, located)
	return started, located, nil
}

func (m *LiveManager) Stop(ctx context.Context, instanceID string) (cockpit.Instance, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return cockpit.Instance{}, fmt.Errorf("runtime stop requires instance id")
	}
	stopped, err := m.cockpit.StopInstance(ctx, instanceID)
	if err != nil {
		return cockpit.Instance{}, err
	}
	m.Forget(instanceID)
	return stopped, nil
}

func (m *LiveManager) Bridge(instanceID string) (antigravityide.LocatedBridge, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bridge, ok := m.bridges[instanceID]
	return bridge, ok
}

func (m *LiveManager) Forget(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.bridges, instanceID)
}

func (m *LiveManager) remember(instanceID string, bridge antigravityide.LocatedBridge) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bridges[instanceID] = bridge
}
