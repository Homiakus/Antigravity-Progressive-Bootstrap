package session

import "github.com/homiakus/agctl/internal/antigravityide"

// Bridge returns the live authenticated Bridge owned by this long-lived
// session service. Credentials are intentionally never reconstructed from
// registration files: only instances started/owned by this process can be
// controlled after the security boundary has been established.
func (s *Service) Bridge(instanceID string) (antigravityide.LocatedBridge, bool) {
	if s == nil {
		return antigravityide.LocatedBridge{}, false
	}
	return s.liveBridge(instanceID)
}
