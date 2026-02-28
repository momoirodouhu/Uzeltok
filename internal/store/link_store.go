package store

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
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

// ListLinks は baseDir 内の全てのリンクを取得してリストとして返します
func (s *LinkStore) ListLinks() ([]*model.Link, error) {
    entries, err := os.ReadDir(s.baseDir)
    if err != nil {
        if os.IsNotExist(err) {
            return []*model.Link{}, nil
        }
        return nil, err
    }

    var links []*model.Link
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
            // 個別のメタデータ読込に失敗した場合はスキップ
            continue
        }
        links = append(links, l)
    }

    return links, nil
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

    // リンク専用のディレクトリを基準とする
    expectedBase := filepath.Join(baseAbs, l.ID)
    p := filepath.Join(expectedBase, filename)
    p = filepath.Clean(p)

    // expectedBase を基準に相対パスを取得し、".." で上方向に出ていないことを確認
    rel, err := filepath.Rel(expectedBase, p)
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

// SaveFile はリンクディレクトリにファイルを書き込みます。
// パスバリデーションは OpenFile と同等です。
func (s *LinkStore) SaveFile(linkID, filename string, r io.Reader) error {
    if linkID == "" || filename == "" {
        return ErrInvalidPath
    }
    if filepath.IsAbs(filename) {
        return ErrInvalidPath
    }

    baseAbs, err := filepath.Abs(s.baseDir)
    if err != nil {
        return err
    }

    expectedBase := filepath.Join(baseAbs, linkID)
    p := filepath.Join(expectedBase, filename)
    p = filepath.Clean(p)

    rel, err := filepath.Rel(expectedBase, p)
    if err != nil {
        return ErrInvalidPath
    }
    if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
        return ErrInvalidPath
    }

    f, err := os.Create(p)
    if err != nil {
        return err
    }
    defer f.Close()

    _, err = io.Copy(f, r)
    return err
}

// DeleteFile はリンクディレクトリからファイルを削除します。
// パスバリデーションは OpenFile と同等です。
func (s *LinkStore) DeleteFile(linkID, filename string) error {
    if linkID == "" || filename == "" {
        return ErrInvalidPath
    }
    if filepath.IsAbs(filename) {
        return ErrInvalidPath
    }

    baseAbs, err := filepath.Abs(s.baseDir)
    if err != nil {
        return err
    }

    expectedBase := filepath.Join(baseAbs, linkID)
    p := filepath.Join(expectedBase, filename)
    p = filepath.Clean(p)

    rel, err := filepath.Rel(expectedBase, p)
    if err != nil {
        return ErrInvalidPath
    }
    if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
        return ErrInvalidPath
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
