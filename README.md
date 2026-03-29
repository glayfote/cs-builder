# CS Builder

- ドキュメント索引: [docs/README.md](docs/README.md)
- 要件・スコープ: [docs/requirements.md](docs/requirements.md)
- 設定 YAML 仕様: [docs/config-spec.md](docs/config-spec.md)
- リポジトリ構成: [docs/directory.md](docs/directory.md)
- アーキテクチャ: [docs/architecture.md](docs/architecture.md)

## 開発メモ

- **実装済み**: 設定読込、`scan`（`.sln` 列挙）、TTY 時の対話ウィザードと依存順 `dotnet build`（`buildorder`）、`dotnet` パス解決。
- **未実装**: `cs-builder build` サブコマンド（スタブ）、設定の `log` / `artifacts` をビルドフローへ接続。

```bash
cd testdata/monorepo
go run ../.. scan --config cs-builder.yaml --verbose
```

対話ビルドはリポジトリルートなど TTY がある場所で `go run .`（引数なし）。既定ではカレントディレクトリの `cs-builder.yaml` を読む。`--config` または環境変数 `CS_BUILDER_CONFIG` で上書き。
