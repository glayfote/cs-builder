// Package config は .cs-builder.toml 設定ファイルの読み込みを担当する。
//
// 設定ファイルは指定ディレクトリから親方向に探索され、
// 最初に見つかった .cs-builder.toml が使用される。
// 見つからない場合はゼロ値の Config を返し、全てデフォルト値で動作する。
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const fileName = ".cs-builder.toml"

// Config は .cs-builder.toml のトップレベル構造を表す。
type Config struct {
	Scan     ScanConfig     `toml:"scan"`
	Commands CommandsConfig `toml:"commands"`
	Defaults DefaultsConfig `toml:"defaults"`
	Log      LogConfig      `toml:"log"`

	// Dir は設定ファイルが見つかったディレクトリの絶対パス。
	// scan.project_root の相対パス解決に使用する。
	// 設定ファイルが見つからなかった場合は空文字列。
	Dir string `toml:"-"`
}

// LogConfig は構造化ログの出力に関する設定を保持する。
type LogConfig struct {
	Dir      string `toml:"dir"`       // ログ出力ディレクトリ (デフォルト: "logs")
	Level    string `toml:"level"`     // ログレベル: debug/info/warn/error (デフォルト: "info")
	BuildLog bool   `toml:"build_log"` // true なら MSBuild/dotnet の全文を <stem>-build.txt に追記
}

// ScanConfig はスキャン動作に関する設定を保持する。
type ScanConfig struct {
	ProjectRoot string     `toml:"project_root"` // プロジェクトルート (設定ファイルからの相対パス)
	Roots       []ScanRoot `toml:"roots"`         // スキャン対象サブディレクトリとコピー先の設定
	Exclude     []string   `toml:"exclude"`       // 追加の除外パターン
}

// ScanRoot はスキャン対象のサブディレクトリとビルド成果物のコピー先を保持する。
type ScanRoot struct {
	Path         string `toml:"path"`           // スキャン対象ディレクトリ (project_root からの相対パス)
	SharedDllDir string `toml:"shared_dll_dir"` // ビルド成果物のコピー先 (project_root からの相対パス、空ならコピーしない)
}

// CommandsConfig はビルドコマンドの実行パスを保持する。
// 空文字列の場合は PATH から探す。
type CommandsConfig struct {
	Dotnet  string `toml:"dotnet"`
	MSBuild string `toml:"msbuild"`
}

// DefaultsConfig は CLI フラグのデフォルト値を上書きする設定を保持する。
type DefaultsConfig struct {
	BuildCmd    string `toml:"build_cmd"`
	Config      string `toml:"config"`
	MaxParallel int    `toml:"max_parallel"` // 同一レベル内の最大並列ビルド数 (0=無制限, 1=逐次)
}

// Load は startDir から親方向に .cs-builder.toml を探索して読み込む。
// 見つからない場合はゼロ値の Config を返す (エラーではない)。
// ファイルが存在するがパースに失敗した場合はエラーを返す。
func Load(startDir string) (Config, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return Config{}, err
	}

	path := findConfig(abs)
	if path == "" {
		return Config{}, nil
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, err
	}
	cfg.Dir = filepath.Dir(path)

	// scan.project_root を設定ファイルのディレクトリからの絶対パスに解決する
	if cfg.Scan.ProjectRoot != "" && !filepath.IsAbs(cfg.Scan.ProjectRoot) {
		cfg.Scan.ProjectRoot = filepath.Join(cfg.Dir, cfg.Scan.ProjectRoot)
	}

	// scan.roots[].path / scan.roots[].shared_dll_dir は project_root からの
	// 相対パスとして後で解決するため、ここでは生の値をそのまま保持する

	return cfg, nil
}

// findConfig は dir から親ディレクトリを辿りながら .cs-builder.toml を探す。
// 見つかればそのフルパスを返し、ルートまで辿っても見つからなければ空文字列を返す。
func findConfig(dir string) string {
	for {
		candidate := filepath.Join(dir, fileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
