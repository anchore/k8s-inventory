package applications

import "time"

// The types below mirror the subset of the Anchore Enterprise Apps API (/v2/apps) used by the application sync

type SystemMetadata struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

type Pagination struct {
	NextCursor *string `json:"next_cursor"`
}

type App struct {
	Name           string         `json:"name"`
	SystemMetadata SystemMetadata `json:"system_metadata"`
}

type AppVersion struct {
	Name           string         `json:"name"`
	SystemMetadata SystemMetadata `json:"system_metadata"`
}

type Asset struct {
	Name string `json:"name"`
}

type appListResponse struct {
	Pagination Pagination `json:"pagination"`
	Items      []App      `json:"items"`
}

type appVersionListResponse struct {
	Pagination Pagination   `json:"pagination"`
	Items      []AppVersion `json:"items"`
}

type assetListResponse struct {
	Pagination Pagination `json:"pagination"`
	Items      []Asset    `json:"items"`
}

type addContainerImageAssetJobResponse struct {
	Status         string         `json:"status"`
	SystemMetadata SystemMetadata `json:"system_metadata"`
}

type contact struct {
	Name string `json:"name"`
}

type appCreateRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Contact     contact `json:"contact"`
}

type appVersionCreateRequest struct {
	Name              string  `json:"name"`
	Description       string  `json:"description,omitempty"`
	Status            string  `json:"status,omitempty"`
	PreviousVersionID *string `json:"previous_version_id,omitempty"`
	ReleaseDate       string  `json:"release_date,omitempty"`
}
