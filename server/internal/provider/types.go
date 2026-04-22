package provider

import "time"

type Domain struct {
	ZoneID   string `json:"zoneId"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

type DNSRecord struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
	Proxied  *bool  `json:"proxied,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

type RecordMutation struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
	Proxied  *bool  `json:"proxied,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

type ValidationResult struct {
	OK        bool      `json:"ok"`
	Message   string    `json:"message"`
	CheckedAt time.Time `json:"checkedAt"`
}
