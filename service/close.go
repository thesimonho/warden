package service

// Close releases Service resources. Called by the top-level Warden.Close().
func (s *Service) Close() {
	s.StopAllSessionWatchers()
	s.stopAllSocketBridges()
}
