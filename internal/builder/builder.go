// Package builder は C# ソリューションファイル (.sln) のビルド実行を担当する。
//
// dotnet build または msbuild コマンドを外部プロセスとして起動し、
// stdout/stderr の出力をリアルタイムにキャプチャする。
// ビルドの中断は context.Context のキャンセルで制御できる。
package builder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// BuildResult は単一ソリューションのビルド結果を保持する。
// TUI のサマリ表示や、ビルドログの詳細確認に使用される。
type BuildResult struct {
	Solution string        // ビルド対象の .sln ファイルパス
	Success  bool          // ビルドが正常終了したか (exit code == 0)
	Duration time.Duration // ビルド開始から完了までの所要時間
	Output   string        // stdout + stderr の全出力テキスト
}

// BuildOption はビルドコマンドの実行パラメータを保持する。
type BuildOption struct {
	Command       string // 使用するビルドコマンド ("dotnet" または "msbuild")
	Configuration string // ビルド構成 ("Debug" または "Release")
	DotnetPath    string // dotnet コマンドのフルパス (空なら PATH から探す)
	MSBuildPath   string // msbuild コマンドのフルパス (空なら PATH から探す)
}

// Build は指定された .sln ファイルに対してビルドコマンドを実行する。
//
// 動作の流れ:
//  1. BuildOption に基づいてコマンドライン引数を組み立てる
//  2. 外部プロセスを起動し、stdout パイプを取得する
//  3. stderr は stdout に合流させ、単一ストリームとして処理する
//  4. 出力を行単位で読み取り、logCh チャネルに逐次送信する
//  5. プロセス終了を待ち、終了コードから成功/失敗を判定する
//
// logCh はビルドログの各行を TUI に伝達するためのチャネル。
// 呼び出し側が logCh を close する責任を持つ必要はない。
// ctx をキャンセルするとプロセスにシグナルが送られビルドが中断される。
func Build(ctx context.Context, slnPath string, opts BuildOption, logCh chan<- string) BuildResult {
	start := time.Now()
	var output strings.Builder

	// コマンド名と引数を BuildOption から決定する
	name, args := buildCommand(slnPath, opts)

	// context 付きでプロセスを生成 (キャンセル時に自動 kill される)
	cmd := exec.CommandContext(ctx, name, args...)

	// stdout のパイプを取得し、行単位のストリーミング読み取りに使用する
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		msg := fmt.Sprintf("[error] stdout pipe: %v", err)
		logCh <- msg
		output.WriteString(msg + "\n")
		return BuildResult{Solution: slnPath, Success: false, Duration: time.Since(start), Output: output.String()}
	}

	// stderr を stdout と同じパイプに合流させる。
	// MSBuild はエラーメッセージを stderr に出力するため、
	// 両方を統合して時系列順にキャプチャする。
	cmd.Stderr = cmd.Stdout

	// プロセスを非同期に開始する
	if err := cmd.Start(); err != nil {
		msg := fmt.Sprintf("[error] start: %v", err)
		logCh <- msg
		output.WriteString(msg + "\n")
		return BuildResult{Solution: slnPath, Success: false, Duration: time.Since(start), Output: output.String()}
	}

	// stdout から行を逐次読み取り、logCh への送信と output への蓄積を並行して行う
	streamLines(stdout, logCh, &output)

	// プロセスの終了を待機し、終了コードから成功/失敗を判定する。
	// err == nil であれば exit code 0 (ビルド成功) を意味する。
	err = cmd.Wait()
	duration := time.Since(start)
	success := err == nil

	return BuildResult{
		Solution: slnPath,
		Success:  success,
		Duration: duration,
		Output:   output.String(),
	}
}

// buildCommand は BuildOption に応じてコマンド名と引数スライスを構築する。
//
// "msbuild" の場合:
//
//	msbuild <slnPath> /p:Configuration=<cfg> /nologo /v:minimal
//
// それ以外 (デフォルト "dotnet") の場合:
//
//	dotnet build <slnPath> -c <cfg> --nologo
//
// Configuration が空文字の場合は "Debug" をデフォルトとする。
func buildCommand(slnPath string, opts BuildOption) (string, []string) {
	cfg := opts.Configuration
	if cfg == "" {
		cfg = "Debug"
	}

	switch strings.ToLower(opts.Command) {
	case "msbuild":
		name := opts.MSBuildPath
		if name == "" {
			name = "msbuild"
		}
		return name, []string{
			slnPath,
			fmt.Sprintf("/p:Configuration=%s", cfg),
			"/nologo",
			"/v:minimal",
		}
	default:
		name := opts.DotnetPath
		if name == "" {
			name = "dotnet"
		}
		return name, []string{
			"build",
			slnPath,
			"-c", cfg,
			"--nologo",
		}
	}
}

// streamLines は io.Reader から行単位で読み取り、
// 各行を logCh チャネルに送信しつつ buf に蓄積する。
//
// バッファサイズは初期 64KB、最大 1MB に設定しており、
// MSBuild の長い警告・エラー行にも対応する。
// 読み取りエラー (EOF 含む) が発生した時点でループを終了する。
func streamLines(r io.Reader, logCh chan<- string, buf *strings.Builder) {
	scanner := bufio.NewScanner(r)
	// MSBuild は長い診断メッセージを出力することがあるため、
	// デフォルトの 64KB バッファでは不足する場合に備えて最大 1MB を許容する
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line + "\n")
		logCh <- line
	}
}
