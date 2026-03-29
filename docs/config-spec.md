# cs-builder.yaml 仕様（version 1）

`internal/config` の `Config` 構造体および `Load` / `ApplyDefaults` / `Validate` の挙動と対応する。

## 設定ファイルの場所

解決順（いずれか 1 つ）:

1. コマンドライン `--config` に非空パスが渡されたとき、そのパス
2. 環境変数 `CS_BUILDER_CONFIG` が非空のとき、そのパス
3. カレントワーキングディレクトリの `cs-builder.yaml`

## トップレベル

| キー | 型 | 必須 | 説明 |
|------|-----|------|------|
| `version` | int | 任意 | `0` または未指定はバージョンチェックをスキップ可能。`1` 以外の正の値は `Validate` で拒否 |
| `project_root` | string | **必須** | モノレポ等のルート。相対パスは **実行時のカレントディレクトリ** 基準で絶対パス化 |
| `scan_roots` | string の配列 | **必須** | `project_root` からの相対パス。少なくとも 1 要素。各要素は実在ディレクトリであること（探索前に検証） |
| `scan_exclude_dir_names` | string の配列 | 任意 | 走査で **入らない** ディレクトリ名（パス 1 セグメント）。大小無視。**YAML で省略（nil）** のときのみ既定が入る。`[]` と明示すると **除外なし** |
| `log` | オブジェクト | 任意 | 下記。省略時は空に近い既定で補完 |
| `artifacts` | オブジェクト | 任意 | 下記。省略時は空に近い既定で補完 |

### `scan_exclude_dir_names` の既定（省略時のみ）

`bin`, `obj`, `.git`, `node_modules`

## `log`

| キー | 型 | 既定（ApplyDefaults 後） | Validate |
|------|-----|--------------------------|----------|
| `file_enabled` | bool | `false` | — |
| `directory` | string | `"logs"` | — |
| `retention_days` | int | `7` | `>= 1` が必須 |

現行のビルド／スキャン処理ではログファイル出力には未接続。

## `artifacts`

| キー | 型 | 説明 | Validate |
|------|-----|------|----------|
| `copy_enabled` | bool | 成果物コピーの想定フラグ | — |
| `destination` | string | コピー先 | `copy_enabled: true` のとき **必須**（空不可） |

現行のビルド処理ではコピーには未接続。

## 検証エラーの例

- `project_root` が空
- `scan_roots` が空配列
- `version` が正でかつ `1` 以外
- `log.retention_days < 1`
- `artifacts.copy_enabled` が true で `destination` が空

## サンプル

```yaml
version: 1
project_root: "."
scan_roots:
  - "pfm"
```

ルートの [cs-builder.yaml](../cs-builder.yaml) および [testdata/monorepo/cs-builder.yaml](../testdata/monorepo/cs-builder.yaml) も参照。
