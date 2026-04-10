package service

import "context"

// Close releases Service resources. Called by the top-level Warden.Close().
func (s *Service) Close() {
	s.StopAllSessionWatchers()
	s.stopAllSocketBridges()

	// Clean up the bridge firewall chain on the host. No-op on Docker
	// Desktop. Errors are logged but don't block shutdown.
	_ = s.docker.TeardownBridgeFirewall(context.Background())
}
