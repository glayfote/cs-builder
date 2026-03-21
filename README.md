# CS Builder

- 要件: [docs/requirements.md](docs/requirements.md)
- 設定 YAML 仕様: [docs/config-spec.md](docs/config-spec.md)
- リポジトリ構成案: [docs/directory.md](docs/directory.md)

## 開発メモ

設定の読み込みと `.sln` 探索のみ実装済み（`scan` サブコマンド）。

```bash
cd testdata/monorepo
go run ../.. scan --config cs-builder.yaml --verbose
```

既定ではカレントディレクトリの `cs-builder.yaml` を読む。`--config` または環境変数 `CS_BUILDER_CONFIG` で上書き。
