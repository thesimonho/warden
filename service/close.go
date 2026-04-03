package service

// Close releases Service resources. Currently stops all session
// watchers. Called by the top-level Warden.Close().
func (s *Service) Close() {
	s.StopAllSessionWatchers()
}
