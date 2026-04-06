package service

import (
	"time"

	"github.com/thesimonho/warden/db"
)

// costFallbackNegCacheTTL is how long a "no cost data" result is cached
// before retrying the docker exec fallback.
const costFallbackNegCacheTTL = 60 * time.Second

// isCostFallbackSuppressed returns true if a recent docker exec for the
// given project returned no cost data and the TTL hasn't expired.
// Uses RLock for the common read path; upgrades to Lock only when
// evicting an expired entry.
func (s *Service) isCostFallbackSuppressed(key db.ProjectAgentKey) bool {
	s.costFallbackNegCacheMu.RLock()
	ts, ok := s.costFallbackNegCache[key]
	s.costFallbackNegCacheMu.RUnlock()

	if !ok {
		return false
	}
	if time.Since(ts) <= costFallbackNegCacheTTL {
		return true
	}

	// Entry expired — evict under write lock.
	s.costFallbackNegCacheMu.Lock()
	delete(s.costFallbackNegCache, key)
	s.costFallbackNegCacheMu.Unlock()
	return false
}

// setCostFallbackNegCache records that a docker exec cost fallback for the
// given project returned no data. Subsequent calls to isCostFallbackSuppressed
// will return true until the TTL expires.
func (s *Service) setCostFallbackNegCache(key db.ProjectAgentKey) {
	s.costFallbackNegCacheMu.Lock()
	defer s.costFallbackNegCacheMu.Unlock()
	s.costFallbackNegCache[key] = time.Now()
}

// ClearCostFallbackNegCache removes the negative cache entry for a project.
// Called when a JSONL cost event arrives, indicating the container now has
// cost data and the fallback should be re-enabled.
func (s *Service) ClearCostFallbackNegCache(projectID, agentType string) {
	s.costFallbackNegCacheMu.Lock()
	defer s.costFallbackNegCacheMu.Unlock()
	delete(s.costFallbackNegCache, db.ProjectAgentKey{
		ProjectID: projectID,
		AgentType: agentType,
	})
}
