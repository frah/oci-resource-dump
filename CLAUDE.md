# CLAUDE.md

## Project Overview

コマンドが実行されたOCIテナントに存在するリソースの情報をダンプするCLIコマンド
リソースの種類とリソース名、OCID、および各リソースタイプ固有の詳細情報を出力する
設定可能なログレベルで詳細度を制御し、プログレスバーと統計レポートで進捗状況を表示する

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
- 対応リソース：Compute instances, VCNs, Subnets, Block volumes, Object Storage, OKE, DRG, Database Service, Load Balancer, Autonomous Databases, Functions, API Gateway, File Storage Service, Network Load Balancer, Streaming Service
- 各リソースタイプに付加情報を含む詳細情報を提供
- ページネーション、並行処理、リトライ機構でパフォーマンス最適化
- 設定可能なログレベルで出力詳細度を制御
- プログレスバーと統計レポートで進捗状況を表示

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

# Timeout setting
./oci-resource-dump --timeout 60
./oci-resource-dump -t 30

# Log level control
./oci-resource-dump --log-level silent    # エラーのみ
./oci-resource-dump --log-level normal    # 基本情報（デフォルト）
./oci-resource-dump --log-level verbose   # 詳細情報
./oci-resource-dump --log-level debug     # デバッグ情報
./oci-resource-dump -l debug              # ショートハンド

# Progress bar control
./oci-resource-dump --progress            # プログレスバー表示
./oci-resource-dump --no-progress         # プログレスバー非表示

# Statistics report
./oci-resource-dump --stats               # 統計レポート出力
./oci-resource-dump --stats-format json   # 統計レポートをJSON形式で
./oci-resource-dump -s                    # 統計レポート（ショートハンド）

# Combined options
./oci-resource-dump -f csv -l verbose --progress --stats -t 45

# Help
./oci-resource-dump --help
```

## Advanced Features

### Performance Optimization
- **Pagination**: 全リソースタイプで完全なページネーション実装
- **Concurrent Processing**: 最大5コンパートメントの並行処理
- **Retry Mechanism**: 指数バックオフ + ジッター機能付きリトライ
- **Timeout Control**: 設定可能なタイムアウト（デフォルト30分）

### Log Level Control
- **Silent** (`--log-level silent`): エラーメッセージのみ出力
- **Normal** (`--log-level normal`): 基本的な進捗情報（デフォルト）
- **Verbose** (`--log-level verbose`): 詳細な処理情報と統計
- **Debug** (`--log-level debug`): 全ての詳細情報とAPI呼び出し情報

### Progress Visualization
- **Progress Bar** (`--progress`): リアルタイム進捗バー表示
- **ETA Calculation**: 推定残り時間の計算と表示
- **Current Operation**: 現在処理中のコンパートメント/リソースタイプ表示
- **Resource Counters**: リアルタイム発見リソース数表示

### Statistics Report
- **Execution Summary** (`--stats`): 実行時間、API呼び出し回数、スループット
- **Resource Statistics**: リソースタイプ別発見数と処理時間
- **Error Analysis**: エラー/リトライ統計とパフォーマンス分析
- **Format Options**: テキスト、JSON、CSV形式での統計出力

### Supported Resource Types
#### Core Infrastructure
- Compute Instances (プライマリIP、形状)
- Virtual Cloud Networks (CIDR、DNS設定)
- Subnets (CIDR、可用性ドメイン)
- Block Volumes (サイズ、パフォーマンスティア)
- Dynamic Routing Gateways

#### Storage & Object Services
- Object Storage Buckets (ストレージティア)
- File Storage Service (容量、パフォーマンス設定)

#### Container & Compute Services
- Oracle Kubernetes Engine Clusters (Kubernetesバージョン)
- Functions (ランタイム、メモリ設定)

#### Database Services
- Database Systems (形状、エディション)
- Autonomous Databases (ワークロードタイプ、CPU/ストレージ設定)

#### Networking & Load Balancing
- Load Balancers (形状、IPアドレス)
- Network Load Balancers (帯域幅、ターゲット情報)

#### API & Integration Services
- API Gateways (エンドポイント、デプロイメント情報)
- Streaming Service (パーティション数、スループット設定)

## Implementation Status

### ✅ Completed Features
- [x] 基本リソース発見（Compute, VCN, Subnet, Block Volume）
- [x] 拡張リソース発見（Object Storage, OKE, Load Balancer, Database, DRG）
- [x] 付加情報機能（各リソースタイプ固有情報）
- [x] 複数出力形式対応（JSON, CSV, TSV）
- [x] ページネーション実装（全リソースタイプ）
- [x] 並行処理実装（セマフォ制御付き）
- [x] リトライ機構実装（指数バックオフ + ジッター）
- [x] タイムアウト制御実装

### 🚧 Planned Features
- [ ] ログレベル制御機能
- [ ] プログレスバー表示機能
- [ ] 統計レポート機能
- [ ] 追加リソースタイプ（Autonomous DB, Functions, API Gateway, FSS, NLB, Streaming）
- [ ] 設定ファイル対応
- [ ] フィルタリング機能
- [ ] 出力ファイル指定機能

## Notes

- API実行時の認証はinstance principalを使用する
- コマンド引数で出力方式が選択可能 (csv, tsv, json)
- リソースが存在しないものについてはエラーとはせず、出力対象外とする
- 処理進捗状況を標準エラー出力に表示
- 各リソースタイプ固有の詳細情報を付加情報として出力
- 大規模環境でも安定動作するようパフォーマンス最適化済み
