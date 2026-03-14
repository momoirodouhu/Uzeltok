package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"uzeltok/internal/model"
)

type LinkStore struct {
	baseDir string
}

func NewLinkStore(baseDir string) *LinkStore {
	return &LinkStore{baseDir: baseDir}
}

// BaseDir returns the root directory where links and files are stored.
func (s *LinkStore) BaseDir() string {
	return s.baseDir
}

// FilePath returns the validated absolute path for a file in a link directory.
func (s *LinkStore) FilePath(linkID, filename string) (string, error) {
	return s.resolveFilePath(linkID, filename)
}

var (
	ErrNotFound    = errors.New("not found")
	ErrInvalidPath = errors.New("invalid path")
)

// GetLinkMetadata loads only _metadata.json for a link without scanning files.
func (s *LinkStore) GetLinkMetadata(uuid string) (model.Metadata, error) {
	if uuid == "" {
		return model.Metadata{}, ErrNotFound
	}

	metaPath := filepath.Join(s.baseDir, uuid, "_metadata.json")
	b, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Metadata{}, ErrNotFound
		}
		return model.Metadata{}, err
	}

	var md model.Metadata
	if err := json.Unmarshal(b, &md); err != nil {
		return model.Metadata{}, err
	}

	return md, nil
}

// GetLink は baseDir/<uuid>/_metadata.json を探して
// 見つかればパースして返します。見つからなければ ErrNotFound を返します。
func (s *LinkStore) GetLink(uuid string) (*model.Link, error) {
	md, err := s.GetLinkMetadata(uuid)
	if err != nil {
		return nil, err
	}

	l := &model.Link{
		ID:       uuid,
		Metadata: md,
	}

	// ディレクトリを走査して Files を補完する（存在しない場合は空のまま返す）
	dir := filepath.Join(s.baseDir, uuid)
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if name == "_metadata.json" {
				continue
			}
			info, err := e.Info()
			if err != nil {
				log.Printf("warn: stat file %q in link %q: %v", name, uuid, err)
				continue
			}
			l.Files = append(l.Files, model.FileInfo{
				Name:      name,
				Size:      info.Size(),
				Timestamp: info.ModTime(),
			})
		}
	}

	return l, nil
}

// ListLinks は baseDir 内の全てのリンクを取得してリストとして返します。
// 個別エントリの読み込みに失敗した場合は warnings に積み上げ、読み込めたリンクだけ返します。
func (s *LinkStore) ListLinks() ([]*model.Link, []error, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*model.Link{}, nil, nil
		}
		return nil, nil, err
	}

	var (
		links    []*model.Link
		warnings []error
	)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// _trash や隠しディレクトリをスキップ
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		l, err := s.GetLink(name)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("link %q: %w", name, err))
			continue
		}
		links = append(links, l)
	}

	return links, warnings, nil
}

// ファイル実体の取得（ReadCloserを返すことで高速・低メモリ）
func (s *LinkStore) OpenFile(l *model.Link, filename string) (io.ReadCloser, error) {
	if l == nil || filename == "" {
		return nil, ErrInvalidPath
	}
	p, err := s.resolveFilePath(l.ID, filename)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

// CreateLink は baseDir/<linkID>/ ディレクトリと _metadata.json を新規作成します。
func (s *LinkStore) CreateLink(l *model.Link) error {
	if l == nil || l.ID == "" {
		return ErrInvalidPath
	}
	dir := filepath.Join(s.baseDir, l.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(l.Metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "_metadata.json"), b, 0o644)
}

// DeleteFile はリンクディレクトリからファイルを削除します。
// パスバリデーションは OpenFile と同等です。
func (s *LinkStore) DeleteFile(linkID, filename string) error {
	if linkID == "" || filename == "" {
		return ErrInvalidPath
	}
	p, err := s.resolveFilePath(linkID, filename)
	if err != nil {
		return err
	}

	err = os.Remove(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *LinkStore) resolveFilePath(linkID, filename string) (string, error) {
	if filepath.IsAbs(filename) {
		return "", ErrInvalidPath
	}

	baseAbs, err := filepath.Abs(s.baseDir)
	if err != nil {
		return "", err
	}

	expectedBase := filepath.Join(baseAbs, linkID)
	p := filepath.Join(expectedBase, filename)
	p = filepath.Clean(p)

	rel, err := filepath.Rel(expectedBase, p)
	if err != nil {
		return "", ErrInvalidPath
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", ErrInvalidPath
	}
	if rel == "_metadata.json" || strings.HasPrefix(rel, "_metadata.json"+string(os.PathSeparator)) {
		return "", ErrInvalidPath
	}

	return p, nil
}

// DeleteLink はリンクディレクトリ全体を _trash ディレクトリに移動します（論理削除）。
func (s *LinkStore) DeleteLink(uuid string) error {
	if uuid == "" {
		return ErrInvalidPath
	}
	srcDir := filepath.Join(s.baseDir, uuid)

	// リンクが存在するか確認
	if _, err := os.Stat(srcDir); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}

	trashDir := filepath.Join(s.baseDir, "_trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return err
	}

	// タイムスタンプを付けて衝突防止
	destName := fmt.Sprintf("%s_%d", uuid, time.Now().Unix())
	destDir := filepath.Join(trashDir, destName)

	return os.Rename(srcDir, destDir)
}
