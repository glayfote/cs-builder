# リポジトリ構成

## ツリー概要

```
cs-builder/
├── main.go                 # エントリ（cmd.Execute）
├── cmd/                    # Cobra: ルート・scan・build（スタブ）
├── internal/
│   ├── config/             # YAML 読込・既定値・検証
│   ├── scan/               # scan_roots に基づく .sln 探索
│   ├── sln/                # .sln から .csproj パス抽出
│   ├── buildorder/         # 依存グラフとビルド順序
│   ├── dotnetpath/         # dotnet 実行ファイルの解決
│   └── tui/                # Bubble Tea ウィザード
├── testdata/monorepo/      # 手元検証用のモノレポ風レイアウト・設定例
├── docs/                   # 本ドキュメント
├── Dockerfile              # Go 1.26 開発用イメージ（既定 CMD は bash）
├── docker-compose.yml      # ボリュームマウントした開発コンテナ
├── cs-builder.yaml         # リポジトリ直下の設定例（パスは環境に合わせて調整）
├── go.mod / go.sum
└── README.md
```

## パッケージとファイルの対応

| パッケージ | 主なファイル | 役割 |
|------------|--------------|------|
| `cmd` | `root.go`, `scan.go`, `build.go` | CLI 定義、TTY 判定とウィザード起動 |
| `internal/config` | `config.go` | `cs-builder.yaml` |
| `internal/scan` | `scanner.go` | `.sln` 列挙と `Solution` メタデータ |
| `internal/sln` | `sln.go` | ソリューションファイルの軽量パース |
| `internal/buildorder` | `resolve.go`, `index.go`, `csprojparse.go` | 依存解決 |
| `internal/dotnetpath` | `dotnetpath.go` | `dotnet` のパス |
| `internal/tui` | `wizard.go`, `wizard_model.go`, `resolve.go` | 対話 UI とビルド実行 |

テストは各パッケージの `*_test.go` に配置。
