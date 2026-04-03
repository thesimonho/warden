package service

import (
	"fmt"

	"github.com/thesimonho/warden/db"
)

// resolveProject looks up a project row by compound key. Returns ErrNotFound
// when the project doesn't exist, allowing HTTP handlers to map it
// to a 404 response.
func (s *Service) resolveProject(projectID, agentType string) (*db.ProjectRow, error) {
	if s.db == nil {
		return nil, ErrNotFound
	}
	row, err := s.db.GetProject(projectID, agentType)
	if err != nil {
		return nil, fmt.Errorf("looking up project %q/%s: %w", projectID, agentType, err)
	}
	if row == nil {
		return nil, ErrNotFound
	}
	return row, nil
}
