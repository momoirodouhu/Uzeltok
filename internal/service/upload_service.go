package service

import (
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

// UploadService は multipart と tus で共通の保存処理を提供します。
type UploadService struct {
	links *LinkService
}

func NewUploadService(links *LinkService) *UploadService {
	return &UploadService{links: links}
}

func (s *UploadService) SaveMultipartFiles(linkID string, files []*multipart.FileHeader) error {
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			return err
		}

		filename := filepath.Base(strings.ReplaceAll(fh.Filename, "\x00", ""))
		if filename == "" || filename == "." || filename == string(filepath.Separator) {
			_ = f.Close()
			continue
		}

		err = s.saveStream(linkID, filename, f)
		_ = f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *UploadService) PersistTusUpload(linkID, filename, srcPath string) error {
	cleanName := filepath.Base(strings.ReplaceAll(filename, "\x00", ""))
	dstPath, err := s.links.FilePath(linkID, cleanName)
	if err != nil {
		return err
	}

	return copyFileAtomically(srcPath, dstPath)
}

func (s *UploadService) saveStream(linkID, filename string, r io.Reader) error {
	dstPath, err := s.links.FilePath(linkID, filename)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(dstPath), ".uzeltok-upload-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, dstPath); err != nil {
		return err
	}

	cleanup = false
	return nil
}

func copyFileAtomically(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dir := filepath.Dir(dstPath)
	tmp, err := os.CreateTemp(dir, ".uzeltok-upload-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, dstPath); err != nil {
		return err
	}

	cleanup = false
	return nil
}
