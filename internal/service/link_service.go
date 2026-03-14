package service

import (
	"io"

	"uzeltok/internal/model"
	"uzeltok/internal/store"
)

// LinkService はリンク関連のユースケース操作を提供します。
type LinkService struct {
	store *store.LinkStore
}

func NewLinkService(s *store.LinkStore) *LinkService {
	return &LinkService{store: s}
}

func (s *LinkService) BaseDir() string {
	return s.store.BaseDir()
}

func (s *LinkService) GetLink(id string) (*model.Link, error) {
	return s.store.GetLink(id)
}

func (s *LinkService) GetLinkMetadata(id string) (model.Metadata, error) {
	return s.store.GetLinkMetadata(id)
}

func (s *LinkService) ListLinks() ([]*model.Link, []error, error) {
	return s.store.ListLinks()
}

func (s *LinkService) CreateLink(l *model.Link) error {
	return s.store.CreateLink(l)
}

func (s *LinkService) DeleteLink(id string) error {
	return s.store.DeleteLink(id)
}

func (s *LinkService) DeleteFile(linkID, filename string) error {
	return s.store.DeleteFile(linkID, filename)
}

func (s *LinkService) OpenFile(l *model.Link, filename string) (io.ReadCloser, error) {
	return s.store.OpenFile(l, filename)
}

func (s *LinkService) FilePath(linkID, filename string) (string, error) {
	return s.store.FilePath(linkID, filename)
}
