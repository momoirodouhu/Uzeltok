package store

import (
    "encoding/json"
    "errors"
    "io"
    "os"
    "path/filepath"
    "strings"

    "uzeltok/internal/model"
)

type LinkStore struct {
    baseDir string
}

func NewLinkStore(baseDir string) *LinkStore {
    return &LinkStore{baseDir: baseDir}
}

var (
    ErrNotFound    = errors.New("not found")
    ErrInvalidPath = errors.New("invalid path")
)

// GetLink は baseDir/<uuid>/_metadata.json を探して
// 見つかればパースして返します。見つからなければ ErrNotFound を返します。
func (s *LinkStore) GetLink(uuid string) (*model.Link, error) {
    if uuid == "" {
        return nil, ErrNotFound
    }

    metaPath := filepath.Join(s.baseDir, uuid, "_metadata.json")
    b, err := os.ReadFile(metaPath)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, ErrNotFound
        }
        return nil, err
    }
    var md model.Metadata
    if err := json.Unmarshal(b, &md); err != nil {
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

// ファイル実体の取得（ReadCloserを返すことで高速・低メモリ）
func (s *LinkStore) OpenFile(l *model.Link, filename string) (io.ReadCloser, error) {
    if l == nil || filename == "" {
        return nil, ErrInvalidPath
    }
    // 基本的なサニタイズ: 絶対パスを拒否し、解決後のパスが baseDir 内に留まることを確認
    if filepath.IsAbs(filename) {
        return nil, ErrInvalidPath
    }

    baseAbs, err := filepath.Abs(s.baseDir)
    if err != nil {
        return nil, err
    }

    // 結合して正規化した絶対パスを作成
    p := filepath.Join(baseAbs, l.ID, filename)
    p = filepath.Clean(p)

    // baseAbs を基準に相対パスを取得し、".." で上方向に出ていないことを確認
    rel, err := filepath.Rel(baseAbs, p)
    if err != nil {
        return nil, ErrInvalidPath
    }
    if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
        return nil, ErrInvalidPath
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
