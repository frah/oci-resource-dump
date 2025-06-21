# CLAUDE.md

## Project Overview

コマンドが実行されたOCIテナントに存在するリソースの情報をダンプするCLIコマンド
リソースの種類とリソース名、OCID、および各リソースタイプ固有の詳細情報を出力する
処理中の進捗状況をリアルタイムで表示する

## Development Setup

### Prerequisites
- Go 1.19 or later
- OCI instance with instance principal authentication configured

### Build
```bash
go build -o oci-resource-dump main.go
```

### Dependencies
- github.com/oracle/oci-go-sdk/v65 (OCI Go SDK)

## Architecture

- Go言語で記載されたCLIコマンドである
- OCI Go SDKを使用してリソース情報を取得
- Instance Principal認証で OCI APIにアクセス
- 複数のコンパートメントを自動的に検索
- 対応リソース：Compute instances, VCNs, Subnets, Block volumes, Object Storage, OKE, DRG, Exadata Cloud Service, Base Database Service, Load Balancer
- 各リソースタイプに付加情報を含む詳細情報を提供
- 処理中の進捗フィードバック機能を搭載

## Commands

### Build
```bash
go build -o oci-resource-dump main.go
```

### Run
```bash
# JSON output (default)
./oci-resource-dump

# CSV output
./oci-resource-dump --format csv
./oci-resource-dump -f csv

# TSV output
./oci-resource-dump --format tsv
./oci-resource-dump -f tsv

# Help
./oci-resource-dump --help
```

## Notes

- API実行時の認証はinstance principalを使用する
- コマンド引数で出力方式が選択可能 (csv, tsv, json)
- リソースが存在しないものについてはエラーとはせず、出力対象外とする
- 処理進捗状況を標準エラー出力に表示
- 各リソースタイプ固有の詳細情報を付加情報として出力
