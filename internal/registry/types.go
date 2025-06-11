package registry

import "time"

type Index struct {
	Version   string             `json:"version"`
	UpdatedAt time.Time          `json:"updated_at"`
	Plugins   map[string]*Plugin `json:"plugins"`
}

type Plugin struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	HomeURL     string            `json:"home_url"`
	Category    string            `json:"category"`
	Tags        []string          `json:"tags"`
	Versions    map[string]*Version `json:"versions"`
	Latest      string            `json:"latest"`
}

type Version struct {
	Version     string    `json:"version"`
	ReleaseDate time.Time `json:"release_date"`
	DownloadURL string    `json:"download_url"`
	SHA256      string    `json:"sha256"`
	Size        int64     `json:"size"`
	MinSupVersion string  `json:"min_sup_version,omitempty"`
}

type PluginInfo struct {
	Name        string
	Version     string
	Author      string
	Description string
	HomeURL     string
	Category    string
	Tags        []string
	Installed   bool
	Available   bool
}