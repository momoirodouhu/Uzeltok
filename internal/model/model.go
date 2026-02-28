package model

import "time"

type LinkType string

const (
	TypeShare LinkType = "share"
	TypeDrop  LinkType = "drop"
)

type Link struct {
	ID       string
	Metadata Metadata
	Files    []FileInfo
}

func (l *Link) IsExpired() bool {
	if l.Metadata.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(l.Metadata.ExpiresAt)
}

type Metadata struct {
	CreatedAt    time.Time `json:"created_at"`
	Type         LinkType  `json:"type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	PasswordHash string    `json:"password_hash,omitempty"`
}

type FileInfo struct {
	Name      string
	Size      int64
	Timestamp time.Time
}
