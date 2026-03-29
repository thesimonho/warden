package api

import "github.com/thesimonho/warden/access"

// --- Access item request/response types ---

// AccessItemResponse is an access item enriched with host detection status.
type AccessItemResponse struct {
	access.Item
	// Detection holds per-credential availability on the current host.
	Detection access.DetectionResult `json:"detection"`
}

// AccessItemListResponse holds the list of all access items.
type AccessItemListResponse struct {
	Items []AccessItemResponse `json:"items"`
}

// CreateAccessItemRequest holds the fields for creating a user-defined access item.
type CreateAccessItemRequest struct {
	Label       string              `json:"label"`
	Description string              `json:"description"`
	Credentials []access.Credential `json:"credentials"`
}

// UpdateAccessItemRequest holds the fields for updating a user-defined access item.
type UpdateAccessItemRequest struct {
	Label       *string              `json:"label"`
	Description *string              `json:"description"`
	Credentials *[]access.Credential `json:"credentials"`
}

// ResolveAccessItemsRequest specifies which access items to resolve.
type ResolveAccessItemsRequest struct {
	Items []access.Item `json:"items"`
}

// ResolveAccessItemsResponse holds the resolution output for a set of access items.
type ResolveAccessItemsResponse struct {
	Items []access.ResolvedItem `json:"items"`
}
