package models

// Host represents a host in the network
type Host struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	AddedAt string `json:"added_at"`
}

// HostWithStatus represents a host with its current status
type HostWithStatus struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	ActualHostname string     `json:"actual_hostname,omitempty"`
	Address        string     `json:"address"`
	Status         string     `json:"status"`
	LastSeen       string     `json:"last_seen,omitempty"`
	IsSelf         bool       `json:"is_self,omitempty"`
	Stats          *HostStats `json:"stats,omitempty"`
}

// HostsOverview represents the hosts overview response
type HostsOverview struct {
	Hosts []HostWithStatus `json:"hosts"`
}
