# octoshell

`octoshell` is a Go CLI that fetches electricity usage data from the Octopus Energy Japan GraphQL API and aggregates half-hourly readings into monthly totals.

It uses Octopus Energy Japan's `halfHourlyReadings` flow to build monthly totals:

1. Obtain a Kraken token with `obtainKrakenToken`, or provide an existing token.
2. Fetch account properties and electricity supply point agreements.
3. Request `halfHourlyReadings` month by month from the earliest agreement start date to the current month.
4. Aggregate each property's readings into monthly usage totals.

See the Japanese guide in [README-JP.md](./README-JP.md).

## Features

- Works with Octopus Energy Japan's GraphQL endpoint
- Supports direct token authentication or email/password login
- Aggregates `halfHourlyReadings` into monthly totals per property
- Outputs either `json` or `csv`
- Supports `-v` for GraphQL request and error debugging with sensitive values redacted

## Requirements

- Go 1.26 or later
- Octopus Energy Japan account number
- Either:
  - a Kraken token
  - Octopus login email and password

## Installation

Build the CLI locally:

```bash
go build -o dist/octoshell ./cmd/octoshell
```

Or run it directly with `go run`.

## Usage

Use an existing token:

```bash
go run ./cmd/octoshell \
  --account-number 'YOUR_ACCOUNT_NUMBER' \
  --token 'YOUR_KRAKEN_TOKEN' \
  --format json
```

Obtain a token with email and password:

```bash
go run ./cmd/octoshell \
  --account-number 'YOUR_ACCOUNT_NUMBER' \
  --email 'YOUR_EMAIL' \
  --password 'YOUR_PASSWORD' \
  --format csv
```

Enable verbose logging:

```bash
go run ./cmd/octoshell \
  -v \
  --account-number 'YOUR_ACCOUNT_NUMBER' \
  --token 'YOUR_KRAKEN_TOKEN'
```

## Options

- `--account-number`: Required. Octopus account number
- `--token`: Existing Kraken token used as the `Authorization` header
- `--email`: Used when `--token` is not provided
- `--password`: Used when `--token` is not provided
- `--property-id`: Restrict output to a single property
- `--timezone`: Aggregation timezone. Default: `Asia/Tokyo`
- `--endpoint`: GraphQL endpoint. Default: `https://api.oejp-kraken.energy/v1/graphql/`
- `--format`: `json` or `csv`
- `--version`: Print the embedded version and exit
- `-v`: Print verbose GraphQL debug logs to stderr

## Output

`json` output returns an array of monthly usage records:

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

`csv` output uses the columns:

`property_id,month,start_at,end_at,read_at,value,unit,type_name`

## Development

Run formatting and tests:

```bash
gofmt -w ./cmd ./internal
go test ./...
```
