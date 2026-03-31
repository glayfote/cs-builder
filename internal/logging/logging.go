// Package logging は log/slog のグローバルロガーを
// JSON 形式のファイル出力で初期化する機能を提供する。
//
// Bubble Tea TUI が stderr を占有するため、ログは常にファイルに出力する。
// ログファイルは実行ごとに YYYY-MM-DD_hhmmss.log の名前で新規作成される。
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultDir = "logs"

// Setup はログディレクトリの作成、ログファイルの生成、
// slog グローバルロガーの設定を行う。
//
// dir が空の場合は "logs" をデフォルトとして使用する。
// level は "debug", "info", "warn", "error" を受け付け、
// 空文字列またはマッチしない場合は INFO になる。
//
// 戻り値の *os.File は呼び出し側で defer Close() する。
func Setup(dir string, level string) (*os.File, error) {
	if dir == "" {
		dir = defaultDir
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("ログディレクトリの作成に失敗: %w", err)
	}

	filename := time.Now().Format("2006-01-02_150405") + ".log"
	path := filepath.Join(dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("ログファイルの作成に失敗: %w", err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	slog.SetDefault(slog.New(handler))

	return f, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
