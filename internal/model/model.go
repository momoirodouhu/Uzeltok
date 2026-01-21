package model

import "time"

type LinkType string

const (
	TypeShare LinkType = "share"
	TypeDrop  LinkType = "drop"
)

type Link struct {
	ID       string
	Type     LinkType
	Metadata Metadata
	Files    []FileInfo
}

type Metadata struct {
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	PasswordHash string    `json:"password_hash,omitempty"`
}

type FileInfo struct {
	Name      string
	Size      int64
	Timestamp time.Time
}