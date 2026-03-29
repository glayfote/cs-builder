# 要件・スコープ（実装ベース）

## 目的

C# モノレポ向けに、設定ファイル（`cs-builder.yaml`）で定義した探索ルートから `.sln` を見つけ、対話または非対話でビルド作業を補助する CLI ツールである。

## 対象ユーザー・利用シナリオ

- **対話（TTY）**: 引数なしで `cs-builder` を実行し、ターミナル上のウィザードで構成（Debug/Release）、テナント、`.sln` を選び、`dotnet build` を依存順で実行する。
- **非対話（CI・スクリプト）**: `scan` サブコマンドで見つかった `.sln` の絶対パスを標準出力に列挙し、シェル等からパイプする。

## 実装済み

| 機能 | 説明 |
|------|------|
| 設定読込 | `--config` → 環境変数 `CS_BUILDER_CONFIG` → カレントの `cs-builder.yaml` の順で解決（`internal/config`） |
| パス検証 | `project_root` と各 `scan_roots` が存在しディレクトリであること（`ValidatePaths`） |
| `.sln` 探索 | `scan_roots` 配下の DFS、除外ディレクトリ、`PackageDir` / `Tenant` ラベル付け（`internal/scan`） |
| `scan` | 探索結果を 1 行 1 パスで stdout、`--verbose` で stderr に補足 |
| 対話ウィザード | Bubble Tea（`internal/tui`）：構成 → テナント → `.sln` 複数選択 → 確認 → ビルド → サマリ |
| ビルド順序 | 選択 `.sln` 内の `.csproj` を起点に `ProjectReference` と `HintPath` の `.dll`（AssemblyName 突合）から DAG を構築し、トポロジカルソート（`internal/buildorder`） |
| `dotnet` 解決 | PATH、`CS_BUILDER_DOTNET`、`DOTNET_ROOT`、Windows の既定パス（`internal/dotnetpath`） |
| `.sln` 解析 | `Project(...)` 行から `.csproj` 相対パスを正規表現で抽出（`internal/sln`）。完全な MSBuild パーサではない |

## 未実装・スタブ

| 項目 | 状態 |
|------|------|
| `cs-builder build` | メッセージのみ（非対話一括ビルドの予約） |
| 設定の `log` / `artifacts` | YAML 上は読み込み・検証されるが、ビルドフローでは未使用 |
| パッケージ／フォルダ単位の UI 行 | `internal/tui/resolve.go` に `buildPackagePickEntries` / `buildFolderPickEntries` があるが、ウィザードは現状 **`.sln` 1 行ずつ** のみ使用 |

## 制約・前提

- ルートコマンドのウィザードは **標準入力が TTY** のときのみ起動する。それ以外はエラーメッセージでサブコマンド利用を促す。
- 同一ディレクトリに `.sln` がある場合、そのディレクトリ以下には探索で降りない（子に別 `.sln` が無い前提の枝刈り）。
- `Tenant` は `scan_root` から `.sln` の親までの相対パスから導出される。ウィザードのテナント選択肢はコード定数（`tenant1` … `tenant4` と `all`）に固定されている。
