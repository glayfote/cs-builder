# ディレクトリ構成（検討案）

Go 製 **cs-builder**（Cobra + Bubble Tea）のリポジトリレイアウト案。要件は [requirements.md](requirements.md)、設定 YAML は [config-spec.md](config-spec.md)。

---

## 1. 方針

- **`internal/`** にアプリ本体を閉じ、**再利用可能なライブラリとして公開しない**前提（標準的な Go の切り方）。
- **Cobra**: エントリは `main.go` → `cmd` パッケージ（サブコマンドは必要になったら分割）。
- **設定・スキャン・dotnet 実行・ログ・生成物コピー・TUI** をパッケージ単位で分離し、テストしやすくする。
- リポジトリ直下の **Dockerfile / docker-compose.yml / Makefile** は現状どおり維持可能。

---

## 2. ツリー（案）

```text
/app/   （リポジトリルート）
├─ go.mod
├─ go.sum
├─ main.go                 # main; cmd.Execute() 等
├─ README.md
├─ Dockerfile
├─ docker-compose.yml
├─ Makefile
├─ .gitignore
│
├─ cmd/                    # Cobra: ルートコマンド・サブコマンド定義のみ（薄く保つ）
│  ├─ root.go
│  └─ build.go             # 必要なら list.go, version.go などを追加
│
├─ internal/
│  ├─ config/              # cs-builder.yaml の読込・検証・デフォルト値
│  │  └─ config.go         # （分割時）load.go, validate.go
│  │
│  ├─ scan/                # project_root / scan_roots に基づく .sln 探索
│  │  └─ scanner.go
│  │
│  ├─ dotnet/              # dotnet CLI の起動、引数組み立て、ストリーム処理
│  │  └─ build.go
│  │
│  ├─ artifacts/           # ビルド成功後の生成物コピー（要件 2.5 / config artifacts）
│  │  └─ copy.go
│  │
│  ├─ logx/                # slog の初期化、セッション 1 ファイル、保持日数による掃引
│  │  └─ session.go        # パッケージ名 log は標準と紛らわしいため logx 等に回避
│  │
│  ├─ tui/                 # Bubble Tea: model / update / view（必要なら keymap）
│  │  ├─ model.go
│  │  ├─ update.go
│  │  └─ view.go
│  │
│  └─ app/                 # （任意）上記を束ねるオーケストレーション「1 フローの実行」
│     └─ run.go            # 無ければ cmd から internal を直呼びでも可
│
├─ docs/
│  ├─ requirements.md
│  ├─ config-spec.md
│  ├─ directory.md         # 本ファイル
│  └─ memo.md
│
├─ examples/               # （任意）リポジトリ用サンプル設定
│  └─ cs-builder.yaml
│
└─ testdata/               # scanner 等のテスト用ディレクトリツリー
   └─ layout01/
      ├─ 1_core/
      ├─ 2_if/
      └─ 3_driver/
```

---

## 3. パッケージの役割（対応表）

| パッケージ | 責務 |
|------------|------|
| `cmd` | フラグ定義、設定読込のトリガー、TTY 判定で TUI 起動か非対話か分岐 |
| `internal/config` | [config-spec.md](config-spec.md) に沿った構造体と YAML アンマーシャル |
| `internal/scan` | 第2層パッケージ / テナント配下の `.sln` 解決 |
| `internal/dotnet` | `dotnet build` 実行と stdout/stderr の取り扱い（ログ連携） |
| `internal/artifacts` | `artifacts.destination` へのコピー規則の実装 |
| `internal/logx` | slog、フロー単位ログファイル、7 日掃引 |
| `internal/tui` | Bubble Tea UI |
| `internal/app` | （任意）「スキャン → 選択 → ビルド → コピー」を 1 連の関数にまとめる |

---

## 4. あえて置かないもの（初期）

- **`pkg/`**: 外部公開ライブラリが不要なら作らない。
- **`bin/`**: ビルド成果物は `.gitignore` で除外し、リポジトリにはコミットしない運用が一般的。
- **`configs/` 固定**: 設定はユーザーリポジトリ側の `cs-builder.yaml` が主。サンプルのみ `examples/` で十分。

---

## 5. 変種（規模が小さいうち）

- `internal/app` を置かず、`cmd/build.go` から `scan` / `dotnet` / `artifacts` を直列に呼ぶ。
- `filter`（TUI 用の絞り込み）を `internal/tui` 内の小さな型で足すか、`internal/scan` に載せるかは実装時に決める。

---

## 6. 次のアクション

この案で問題なければ、**空の `internal/*` ディレクトリとプレースホルダ `.go`** を段階的に追加するか、最初に **config + scan** から実装を始めると依存が少ない。
