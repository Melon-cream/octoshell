# 変更履歴

このファイルには、このプロジェクトの重要な変更を記録します。

形式は Keep a Changelog を参考にし、バージョンは Semantic Versioning を想定しています。

## [0.1.0] - 2026-03-15

### 追加

- Octopus Energy Japan の電気使用量取得用 Go CLI の初版実装
- token 認証と email/password からの token 取得に対応した GraphQL クライアント
- `halfHourlyReadings` を使った月次集計ロジック
- JSON / CSV 出力対応
- 機密値を伏せた verbose GraphQL デバッグ表示
- CLI 引数解析、集計処理、GraphQL 異常系を含む静的テスト
- 英語版 / 日本語版の公開向けドキュメント
- push 時検証、release note 同期、バイナリ配布用の GitHub Actions
