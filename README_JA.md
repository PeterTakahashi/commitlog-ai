# aitrace

> すべてのgitコミットの裏にあるプロンプトを見る

**aitrace** は、Claude Codeの会話ログをgitの履歴に紐づけ、AI支援開発の全体像をチームで共有するためのツールです。

## 課題

Claude Codeを日常的に使っていても、`git log`には*何が*変わったかしか記録されません。*なぜ*変更したのか、*どのプロンプト*で、*どのモデル*を使ったのかは分かりません。ログは隠しディレクトリに散在し、gitとの紐づけも存在しません。

## 解決策

aitraceはClaude Codeのログを読み取り、統一フォーマットに変換し、タイムスタンプでgitコミットとマッチングし、Web UIで閲覧できるようにします。ログ内のAPIキーやシークレットは自動的にマスクされます。

```
Claude Code ログ ──▶ aitrace parse ──▶ aitrace link ──▶ aitrace serve
                                                             │
                                                    localhost:3000
                                              タイムライン + 会話 + Diff
```

## クイックスタート

```bash
go install github.com/anthropics/aitrace@latest

cd your-project
aitrace parse        # Claude Codeログを読み取り → 統一フォーマットへ変換
aitrace link         # セッションとgitコミットを紐づけ
aitrace serve        # Web UIを起動 (localhost:3000)
```

## 対応エージェント

| エージェント | 対応状況 | ログの場所 |
|-------------|---------|-----------|
| Claude Code | 対応済み | `~/.claude/projects/` |

## 機能

- **APIキーの自動マスク** — OpenAI、Anthropic、AWS、Azure、GitHubトークンなどのシークレットを自動検出し、出力ファイルでマスク
- **Gitユーザー情報** — 各コミットにauthorの名前・メールアドレス・GitHubプロフィールアイコンを表示
- **ポート自動フォールバック** — 3000番ポートが使用中の場合、空きポートを自動選択
- **サーバーサイドページネーション** — 数千コミットのリポジトリにも対応
- **Markdownエクスポート** — タイムライン全体を1枚のMarkdownファイルに出力

## コマンド

### `aitrace status`

現在のプロジェクトで検出されたログソースと件数を表示します。

```
$ aitrace status
Project: /Users/you/dev/myproject

  claude_code   3 log file(s)
```

### `aitrace parse`

検出されたすべてのエージェントログを統一JSONフォーマットに変換します。シークレットは自動的にマスクされます。出力は `.aitrace/sessions.json` に書き込まれます。

```
$ aitrace parse
[claude_code] Found 3 log file(s)
  Session a1b2c3d4: 42 messages (09:15:30 to 10:22:45)
  Session e5f6g7h8: 18 messages (14:00:12 to 14:35:20)

Parsed 2 session(s) → .aitrace/sessions.json
```

### `aitrace link`

パースされたセッションをタイムスタンプベースのヒューリスティクスでgitコミットとマッチングします。出力は `.aitrace/timeline.json` に書き込まれます。

```
$ aitrace link
Found 2 session(s) and 28 commit(s)
Linked 2 pair(s), 28 total entries → .aitrace/timeline.json
```

### `aitrace serve`

紐づけられたタイムラインを閲覧するためのローカルWebサーバーを起動します。デフォルトポートが使用中の場合は、空きポートが自動で選択されます。

```
$ aitrace serve
aitrace server running at http://localhost:3000
  28 timeline entries, 2 sessions
```

オプション:
- `--port <number>` — サーバーポート（デフォルト: 3000、使用中の場合は自動で別ポートにフォールバック）
- `--no-browser` — ブラウザの自動オープンを無効化

### `aitrace export`

紐づけられたタイムラインをJSONまたはMarkdownとしてエクスポートします。

```bash
# JSONバンドル
aitrace export --format json
Exported → .aitrace/output/timeline.json

# Markdownレポート（全会話を含む1ファイル）
aitrace export --format markdown
Exported → .aitrace/output/timeline.md
```

オプション:
- `--format json` — JSONバンドル（デフォルト）
- `--format markdown` — サマリー、コミット詳細、会話全文を折りたたみセクションで含む1枚のMarkdownファイル

## マッチングの仕組み

aitraceは信頼度スコア付きのアルゴリズムでセッションとコミットを紐づけます:

1. **時間重複** — コミットのタイムスタンプがセッションの時間範囲内 → 信頼度90%
2. **セッション後のコミット** — セッション終了後5分以内のコミット → 信頼度70%
3. **セッション前のコミット** — セッション開始前5分以内のコミット → 信頼度50%
4. **ファイルパスの重複ボーナス** — ツールコールで操作されたファイルとコミットの変更ファイルが一致 → +10%
5. **ブランチ一致ボーナス** — セッションのgitブランチが一致 → +5%

マッチしなかったコミットやセッションは単独エントリとして表示されます。

## Web UI

Webビューアは3つのビューを提供します:

- **タイムライン** — git log風の一覧表示。インフィニットスクロール、サーバーサイドページネーション、エージェントフィルタ、全文検索対応。各エントリにコミットauthorのGitHubアバター・名前・変更ファイル数を表示。
- **セッション詳細** — 左に会話全文（「You」ではなくコミットauthorの名前を表示）、右にgit diffの分割ビュー。ツール承認メッセージは承認したツール名とともにインライン表示。
- **統計** — エージェント別セッション数、紐づけ状況、メッセージ総数のダッシュボード

## アーキテクチャ

- **Go CLI** — シングルバイナリ、外部依存ゼロ（データベース不要、Docker不要）
- **React SPA** — Vite + TypeScript + Tailwind CSS v4 + shadcn/uiで構築、`go:embed`でGoバイナリに組み込み
- **JSONベース** — すべてのデータは `.aitrace/` にJSONファイルとして保存。ポータブルでgitフレンドリー。serve時にメモリにロードし、サーバーサイドページネーションで配信。
- **シークレットサニタイザー** — 正規表現ベースでAPIキー・トークン・認証情報を検出し、ファイル書き出し前にマスク

## 開発

```bash
# フルビルド (web + Go)
cd web && npm run build && cd ..
rm -rf internal/server/dist && cp -r web/dist internal/server/dist
go build -o bin/aitrace ./cmd/aitrace/

# 開発モード: Vite dev server + Go APIを別々に起動
cd web && npm run dev          # Viteが:5173で起動（/apiを:3000にプロキシ）
go run ./cmd/aitrace/ serve    # Go APIが:3000で起動
```

### プロジェクト構造

```
cmd/aitrace/           CLIエントリポイントとサブコマンド
internal/
  model/               統一データ型（Session, Message, Timeline）
  parser/              Claude Codeログパーサー
  linker/              Git操作とタイムスタンプベースのマッチング
  exporter/            JSONおよびMarkdownエクスポート
  sanitizer/           APIキー・シークレットのマスク処理
  server/              組み込みReact SPAとページネーション付きAPIのHTTPサーバー
web/                   Reactビューア（Vite + TypeScript + Tailwind + shadcn/ui）
```

## ライセンス

MIT
