# 設定ファイル仕様（YAML）

cs-builder が読み込む設定の形式。**要件の背景**は [requirements.md](requirements.md) を参照。

---

## 1. 形式

- **YAML 1.2**（実装では `gopkg.in/yaml.v3` 等を想定）。
- 文字コードは **UTF-8**。

---

## 2. ファイルの場所と探索順

既定のファイル名は **`cs-builder.yaml`**（リポジトリルートに置く想定）。

読み込みの優先順（**上が優先**。実装時にフラグ名は確定させる）:

1. CLI フラグ（例: `--config <path>`）
2. 環境変数（例: `CS_BUILDER_CONFIG` にファイルパス）
3. **カレントワーキングディレクトリ**にある `cs-builder.yaml`

v1 で「親ディレクトリを辿って探索」するかは実装判断。要件ではリポジトリ直下配置が主想定。

---

## 3. 相対パスの基準

| キー | 相対パスの解釈基準 |
|------|-------------------|
| `project_root` | **カレントワーキングディレクトリ**（cwd）。`.` は「実行時の cwd をルートとする」意味。 |
| `scan_roots` の各要素 | **`project_root` を解決したディレクトリ**からの相対パス。 |
| `log.directory` | **`project_root` を解決したディレクトリ**からの相対パス（推奨）。 |
| `artifacts.destination` | **`project_root` を解決したディレクトリ**からの相対パス（推奨）。 |

`project_root` に絶対パスを書いてもよい（CI や複数作業コピー向け）。

---

## 4. CLI との優先順位

**原則**: 同一項目について **CLI フラグが設定ファイルより優先**。  
（どのキーにフラグを用意するかは実装時に列挙する。）

---

## 5. スキーマ（v1）

### 5.1 トップレベル

| キー | 型 | 必須 | 説明 |
|------|-----|------|------|
| `version` | int | 推奨 | 設定スキーマ版。現状は **`1`**。未知の大きな版はエラーとする方針を推奨。 |
| `project_root` | string | 必須 | モノレポのルート相当。`.` 可。 |
| `scan_roots` | string の配列 | 必須 | 探索の起点ディレクトリ（配下を再帰探索し `.sln` を列挙。`scan_exclude_dir_names` で名前一致の子ディレクトリは入らない）。1 要素以上。 |
| `scan_exclude_dir_names` | string の配列 | 任意 | 走査で**入らない**ディレクトリ名（パスの1セグメント）。大文字小文字は区別しない。省略時は `bin`, `obj`, `.git`, `node_modules` を既定とする。空配列 `[]` を明示すると除外なし。 |
| `log` | object | 任意 | 省略時は「ファイル出力なし」相当のデフォルト（下記 5.2）。 |
| `artifacts` | object | 任意 | 省略時はビルド後の**生成物コピーなし**（下記 5.3）。 |

### 5.2 `log` オブジェクト

| キー | 型 | 必須 | 既定 | 説明 |
|------|-----|------|------|------|
| `file_enabled` | bool | 任意 | `false` | `true` のとき、1 ビルドフローにつき 1 ログファイルへ slog と dotnet 出力を記録。 |
| `directory` | string | 任意 | `logs` | ログファイルを置くディレクトリ（`project_root` 基準の相対可）。`file_enabled: false` でも将来用に保持可。 |
| `retention_days` | int | 任意 | `7` | `directory` 内の古いログファイルを削除するまでの保持日数。 |

削除判定は実装で定める（例: ファイルの**最終更新時刻**、またはファイル名に含むタイムスタンプ）。

### 5.3 `artifacts` オブジェクト（生成物のコピー先）

`dotnet build` 後の**ビルド生成物を別ディレクトリへコピー**するための設定。`dotnet build -o` によるビルド時出力の変更とは別（併用の有無は実装判断）。

| キー | 型 | 必須 | 既定 | 説明 |
|------|-----|------|------|------|
| `copy_enabled` | bool | 任意 | `false` | `true` のとき、ビルド成功後に生成物を `destination` 配下へコピーする。 |
| `destination` | string | 条件付き | （なし） | コピー先の**ルートディレクトリ**。`project_root` 基準の相対パスまたは絶対パス。`copy_enabled: true` のとき**必須**。 |

**コピー対象・配下のディレクトリ構成**（例: ソリューション名ごとのサブフォルダにまとめるか、`bin/<Configuration>/<TargetFramework>/` をどう辿るか）は、MSBuild の既定出力に追従するなど**実装で定義**する。必要になったら後続バージョンで `layout`・`include`・`exclude` 等を追加する。

---

## 6. 検証（推奨）

- `project_root` が解決後に存在しディレクトリであること。
- `scan_roots` の各パスが `project_root` 配下に存在すること（警告のみにするかは実装判断）。
- `scan_roots` が空でないこと。
- `log.retention_days` が 1 以上であること（0 以下はエラー）。
- `artifacts.copy_enabled` が `true` のとき `artifacts.destination` が空でないこと。

---

## 7. 完全例

```yaml
version: 1

project_root: "."

scan_roots:
  - "2_if"
  - "3_driver"
  - "1_core"

log:
  file_enabled: true
  directory: "logs"
  retention_days: 7

artifacts:
  copy_enabled: true
  destination: "dist"
```

---

## 8. 将来拡張（未確定・スキーマ v2 候補）

[requirements.md](requirements.md) 第6節が確定したら、例として次を追加しうる。

- `dotnet`: `configuration`, `no_restore`, `extra_args`, `output`（`dotnet build -o` 相当）など
- `build`: 並列実行、失敗時の継続方針など
- `artifacts`: `layout`（コピー時のサブディレクトリ規則）、`include` / `exclude` パターンなど

v1 実装では **未知のトップレベルキー**を無視するか警告するかを決める（後方互換のため無視が無難）。
