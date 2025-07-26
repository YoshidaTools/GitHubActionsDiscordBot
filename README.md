# GitHub Actions Discord Bot

Discord botでGitHub Actionsのワークフローを操作するGoアプリケーション

## 機能

### テキストコマンド
- `!gh workflows <owner> <repo>` - ワークフロー一覧表示
- `!gh run <owner> <repo> <workflow_id>` - ワークフロー実行
- `!gh status <owner> <repo>` - ワークフロー実行状況表示
- `!gh logs <owner> <repo> <run_id>` - ワークフローログ表示

### スラッシュコマンド
- `/build` - 全ビルドワークフロー実行
- `/build-win` - Windowsビルド実行
- `/build-mac` - macOSビルド実行
- `/build-drive` - AssetImporterビルド実行
- `/code-check` - 静的解析実行

## セットアップ

### 必要な準備

1. **Discord Bot作成**
   - Discord Developer Portalでbotを作成
   - BOT TOKENを取得
   - サーバーに招待

2. **GitHub App作成**
   - GitHub Developer settingsでGitHub Appを作成
   - Actions権限を付与
   - Private keyをダウンロード
   - リポジトリにインストール

### 環境変数設定

`.env.example`をコピーして`.env`を作成し、各値を設定：

```bash
cp .env.example .env
```

### 実行

```bash
go run main.go
```

## 必要な権限

### Discord Bot権限
- Send Messages
- Use Slash Commands
- Embed Links

### GitHub App権限
- Actions: Read & Write
- Contents: Read
- Metadata: Read