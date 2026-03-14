# octoshell

`octoshell` は、Octopus Energy Japan の GraphQL API から電気使用量データを取得し、30分単位の `halfHourlyReadings` を月次使用量に集計する Go CLI です。

Octopus Energy Japan の `halfHourlyReadings` ベースの取得フローに合わせて実装しています。

1. `obtainKrakenToken` で token を取得するか、既存 token を渡す
2. account に紐づく property と electricity supply point の契約開始日を取得する
3. 契約開始月から現在月まで、月単位で `halfHourlyReadings` を取得する
4. property ごとに月次合計へ集計する

英語版は [README.md](./README.md) を参照してください。

## 主な機能

- Octopus Energy Japan の GraphQL エンドポイントに対応
- token 直接指定、または email/password での認証に対応
- `halfHourlyReadings` を property ごとの月次使用量へ集計
- `json` と `csv` の両方を出力可能
- `-v` で GraphQL の query、変数、失敗レスポンスを詳細表示

## 前提

- Go 1.26 以上
- Octopus Energy Japan のアカウント番号
- 次のいずれか
  - Kraken token
  - Octopus ログイン用 email / password

## インストール

ローカルでバイナリを作る場合:

```bash
go build -o dist/octoshell ./cmd/octoshell
```

`go run` で直接実行しても構いません。

## 使い方

token を直接使う場合:

```bash
go run ./cmd/octoshell \
  --account-number 'YOUR_ACCOUNT_NUMBER' \
  --token 'YOUR_KRAKEN_TOKEN' \
  --format json
```

email / password で token を取得する場合:

```bash
go run ./cmd/octoshell \
  --account-number 'YOUR_ACCOUNT_NUMBER' \
  --email 'YOUR_EMAIL' \
  --password 'YOUR_PASSWORD' \
  --format csv
```

詳細ログを出す場合:

```bash
go run ./cmd/octoshell \
  -v \
  --account-number 'YOUR_ACCOUNT_NUMBER' \
  --token 'YOUR_KRAKEN_TOKEN'
```

## オプション

- `--account-number`: 必須。Octopus のアカウント番号
- `--token`: 取得済み Kraken token。`Authorization` ヘッダにそのまま設定
- `--email`: `--token` 未指定時に使用
- `--password`: `--token` 未指定時に使用
- `--property-id`: 特定 property のみに絞る
- `--timezone`: 集計タイムゾーン。既定値は `Asia/Tokyo`
- `--endpoint`: GraphQL エンドポイント。既定値は `https://api.oejp-kraken.energy/v1/graphql/`
- `--format`: `json` または `csv`
- `--version`: 埋め込みバージョンを表示して終了
- `-v`: 詳細ログを stderr に出力

## 出力

`json` の場合は月次使用量の配列を返します。

```json
[
  {
    "propertyId": "123456",
    "month": "2026-03",
    "startAt": "2026-03-01T00:00:00+09:00",
    "endAt": "2026-04-01T00:00:00+09:00",
    "readAt": "2026-04-01T00:00:00+09:00",
    "value": "85.9",
    "typeName": "ElectricityHalfHourReadingAggregate"
  }
]
```

`csv` の場合は次の列を出力します。

`property_id,month,start_at,end_at,read_at,value,unit,type_name`

## 開発

フォーマットとテスト:

```bash
gofmt -w ./cmd ./internal
go test ./...
```

## 注意

- 現時点では静的テストのみ実施しています
- 実 API 確認は手元の認証情報で行ってください
- `outputs/` 配下のローカル出力は誤って公開しないよう Git の管理対象外にしています
