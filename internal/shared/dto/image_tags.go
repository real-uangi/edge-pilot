package dto

import "github.com/google/uuid"

type ListServiceImageTagsRequest struct {
	ServiceID uuid.UUID `json:"serviceId" binding:"required"`
	Cursor    string    `json:"cursor"`
}

type ServiceImageTagsOutput struct {
	ImageRepo  string   `json:"imageRepo"`
	Tags       []string `json:"tags"`
	NextCursor string   `json:"nextCursor"`
}

// ResolvedRegistryCredential is only exchanged between backend domains.
type ResolvedRegistryCredential struct {
	Host     string `json:"-"`
	Username string `json:"-"`
	Secret   string `json:"-"`
}
