package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigFileName はフラグ・環境変数でパスが指定されないとき、カレントディレクトリから探すファイル名。
	DefaultConfigFileName = "cs-builder.yaml"
	// EnvConfigPath は設定ファイルの絶対パスを渡す環境変数名（config-spec / 実装で共有）。
	EnvConfigPath = "CS_BUILDER_CONFIG"
	// SupportedVersion はこのパッケージが解釈できる設定スキーマの版（YAML の version と対応）。
	SupportedVersion = 1
)

// Config は cs-builder.yaml のトップレベルに対応する構造体（config-spec.md v1）。
type Config struct {
	// Version は設定スキーマ版。0 のときは未指定扱いでバージョンチェックをスキップ可。SupportedVersion 以外は Validate で拒否。
	Version int `yaml:"version"`
	// ProjectRoot はモノレポ等のルート。相対パスは実行時のカレントワーキングディレクトリ基準で解決（ResolvedProjectRoot）。
	ProjectRoot string `yaml:"project_root"`
	// ScanRoots は .sln 探索の起点となる、ProjectRoot からの相対パス（複数可）。先頭・末尾の空白は走査側で Trim。
	ScanRoots []string `yaml:"scan_roots"`
	// ScanExcludeDirNames は走査時に入らないディレクトリ名（パス1セグメント）。大小無視。nil のときのみ ApplyDefaults が bin/obj 等を入れる。空スライス [] は「除外なし」。
	ScanExcludeDirNames []string `yaml:"scan_exclude_dir_names,omitempty"`
	// Log はファイルログまわり。省略時は ApplyDefaults で空の LogConfig を確保し、Directory / RetentionDays に既定値。
	Log *LogConfig `yaml:"log,omitempty"`
	// Artifacts はビルド成果物コピー先。省略時は ApplyDefaults で空の ArtifactsConfig を確保。
	Artifacts *ArtifactsConfig `yaml:"artifacts,omitempty"`
}

// LogConfig はログ出力に関する設定。
type LogConfig struct {
	// FileEnabled が true のときビルドフローごとにログファイルへ出力する想定。
	FileEnabled bool `yaml:"file_enabled"`
	// Directory はログを置くディレクトリ（相対なら project_root 基準推奨）。
	Directory string `yaml:"directory"`
	// RetentionDays は古いログを残す日数。1 未満は Validate でエラー。
	RetentionDays int `yaml:"retention_days"`
}

// ArtifactsConfig は dotnet ビルド後の生成物コピー設定。
type ArtifactsConfig struct {
	// CopyEnabled が true のときビルド成功後に生成物をコピーする想定。
	CopyEnabled bool `yaml:"copy_enabled"`
	// Destination はコピー先ルート。CopyEnabled 時は必須（Validate）。
	Destination string `yaml:"destination"`
}

// Load は設定ファイルのパスを決め、読み込み・ApplyDefaults・Validate まで行う。
// explicitPath が空でないときはそのパスのみを読む（例: --config）。戻り値の string は実際に読んだファイルパス。
func Load(explicitPath string) (*Config, string, error) {
	path, err := resolveConfigPath(explicitPath)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, path, fmt.Errorf("parse yaml: %w", err)
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, path, err
	}
	return &cfg, path, nil
}

// ApplyDefaults は省略されたセクションやゼロ値を config-spec の既定で埋める。
// ScanExcludeDirNames は nil のときだけ既定の除外名を設定する（YAML で [] と明示した空は上書きしない）。
func (c *Config) ApplyDefaults() {
	if c.Log == nil {
		c.Log = &LogConfig{}
	}
	if c.Log.Directory == "" {
		c.Log.Directory = "logs"
	}
	if c.Log.RetentionDays == 0 {
		c.Log.RetentionDays = 7
	}
	if c.Artifacts == nil {
		c.Artifacts = &ArtifactsConfig{}
	}
	if c.ScanExcludeDirNames == nil {
		c.ScanExcludeDirNames = []string{"bin", "obj", ".git", "node_modules"}
	}
}

// resolveConfigPath は読むべき設定ファイルの絶対パス（またはクリーン済みパス）を返す。
// 優先順: explicitPath（非空）→ 環境変数 CS_BUILDER_CONFIG → cwd の cs-builder.yaml。
func resolveConfigPath(explicitPath string) (string, error) {
	if strings.TrimSpace(explicitPath) != "" {
		return filepath.Clean(explicitPath), nil
	}
	if env := strings.TrimSpace(os.Getenv(EnvConfigPath)); env != "" {
		return filepath.Clean(env), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return filepath.Join(wd, DefaultConfigFileName), nil
}

// Validate は必須項目と config-spec 上の制約を検査する（Load 内でも呼ばれる）。
func (c *Config) Validate() error {
	if c.Version > 0 && c.Version != SupportedVersion {
		return fmt.Errorf("unsupported config version %d (supported %d)", c.Version, SupportedVersion)
	}
	if strings.TrimSpace(c.ProjectRoot) == "" {
		return errors.New("project_root is required")
	}
	if len(c.ScanRoots) == 0 {
		return errors.New("scan_roots must have at least one entry")
	}
	if c.Log.RetentionDays < 1 {
		return fmt.Errorf("log.retention_days must be >= 1, got %d", c.Log.RetentionDays)
	}
	if c.Artifacts.CopyEnabled && strings.TrimSpace(c.Artifacts.Destination) == "" {
		return errors.New("artifacts.destination is required when artifacts.copy_enabled is true")
	}
	return nil
}

// ResolvedProjectRoot は ProjectRoot を絶対パスに解決する。
// 相対パスはカレントワーキングディレクトリを基準にする（config-spec §3）。
func (c *Config) ResolvedProjectRoot() (string, error) {
	raw := strings.TrimSpace(c.ProjectRoot)
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return filepath.Abs(filepath.Join(wd, raw))
}

// ValidatePaths は project_root および各 scan_roots が実在しディレクトリであることを確認する。
// 探索コマンド等、ディスクアクセス前に失敗させたいときに使う（Validate だけではパス実在は見ない）。
func (c *Config) ValidatePaths() error {
	root, err := c.ResolvedProjectRoot()
	if err != nil {
		return err
	}
	st, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("project_root %q: %w", root, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("project_root %q is not a directory", root)
	}
	for _, rel := range c.ScanRoots {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			return errors.New("scan_roots contains an empty entry")
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		st, err := os.Stat(full)
		if err != nil {
			return fmt.Errorf("scan_roots %q: %w", rel, err)
		}
		if !st.IsDir() {
			return fmt.Errorf("scan_roots %q is not a directory", rel)
		}
	}
	return nil
}
