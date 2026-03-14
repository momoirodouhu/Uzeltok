package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultMaxUploadBytes   int64         = 32 << 20 // 32 MiB
	DefaultTusIncompleteTTL time.Duration = 24 * time.Hour
	DefaultPort                           = "8080"
	DefaultDataDir                        = "./data"
)

// Config はサーバー起動に必要な全設定を保持します。
// 各フィールドは Load() によって環境変数から一括パースされます。
type Config struct {
	DataDir          string
	AdminPassword    string
	MaxUploadBytes   int64
	Port             string
	TusIncompleteTTL time.Duration
}

// Load は環境変数から Config を構築して返します。
// バリデーションに失敗した場合はエラーを返します。
func Load() (Config, error) {
	cfg := Config{
		DataDir:          DefaultDataDir,
		MaxUploadBytes:   DefaultMaxUploadBytes,
		Port:             DefaultPort,
		TusIncompleteTTL: DefaultTusIncompleteTTL,
	}

	if d := os.Getenv("UZELTOK_DATA_DIR"); d != "" {
		cfg.DataDir = d
	}

	cfg.AdminPassword = os.Getenv("ADMIN_PASSWORD")

	if s := os.Getenv("UPLOAD_MAX_BYTES"); s != "" {
		n, err := parseByteSize(s)
		if err != nil {
			return Config{}, fmt.Errorf("invalid UPLOAD_MAX_BYTES: %q (%w)", s, err)
		}
		cfg.MaxUploadBytes = n
	}

	if p := os.Getenv("UZELTOK_PORT"); p != "" {
		if _, err := strconv.Atoi(p); err != nil {
			return Config{}, fmt.Errorf("invalid UZELTOK_PORT: %q", p)
		}
		cfg.Port = p
	}

	if raw := strings.TrimSpace(os.Getenv("TUS_INCOMPLETE_TTL")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			cfg.TusIncompleteTTL = d
		}
	}

	return cfg, nil
}

func parseByteSize(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("value is empty")
	}

	i := 0
	dotCount := 0
	for i < len(s) {
		c := s[i]
		if c >= '0' && c <= '9' {
			i++
			continue
		}
		if c == '.' {
			dotCount++
			if dotCount > 1 {
				return 0, fmt.Errorf("invalid number")
			}
			i++
			continue
		}
		break
	}

	if i == 0 {
		return 0, fmt.Errorf("missing numeric value")
	}

	numPart := s[:i]
	suffix := strings.ToUpper(strings.TrimSpace(s[i:]))

	n, err := strconv.ParseFloat(numPart, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid numeric value")
	}

	mul := float64(1)
	switch suffix {
	case "", "B":
		mul = 1
	case "K", "KB":
		mul = 1000
	case "M", "MB":
		mul = 1000 * 1000
	case "G", "GB":
		mul = 1000 * 1000 * 1000
	case "T", "TB":
		mul = 1000 * 1000 * 1000 * 1000
	case "P", "PB":
		mul = 1000 * 1000 * 1000 * 1000 * 1000
	case "KI", "KIB":
		mul = 1 << 10
	case "MI", "MIB":
		mul = 1 << 20
	case "GI", "GIB":
		mul = 1 << 30
	case "TI", "TIB":
		mul = 1 << 40
	case "PI", "PIB":
		mul = 1 << 50
	default:
		return 0, fmt.Errorf("unsupported size suffix (use bytes or units like MiB/GB)")
	}

	bytes := n * mul
	if bytes < 1 || bytes > float64(math.MaxInt64) {
		return 0, fmt.Errorf("size is out of range")
	}

	return int64(math.Round(bytes)), nil
}
