# Phase 2D: ユニットテスト戦略書

## 概要
OCI Resource Dump CLIツールに対する包括的なユニットテスト実装戦略。コア機能、企業機能、エラーハンドリングの品質保証を目的とする。

## テスト対象モジュール分析

### 現在のモジュール構成（8ファイル）
```
1. main.go          - エントリーポイント、CLI引数処理、モード分岐
2. types.go         - 構造体定義、データ型
3. clients.go       - OCI SDKクライアント管理、認証
4. discovery.go     - リソース発見ロジック、API呼び出し
5. logger.go        - ログ機能、レベル制御
6. progress.go      - プログレス表示、ETA計算
7. output.go        - 出力処理（JSON/CSV/TSV）
8. config.go        - 設定ファイル処理（YAML）
9. filters.go       - フィルタリング機能
10. diff.go         - 差分分析機能
```

## テスト戦略

### 1. テスト方針
- **Unit Tests**: 各モジュールの関数・メソッド単位
- **Table-Driven Tests**: 複数ケース効率的テスト
- **Mock使用**: OCI SDK依存部分のモック化
- **Coverage Goal**: 80%以上のカバレッジ目標

### 2. テスト優先度

#### High Priority（必須）
- **config.go**: 設定読み込み、YAML解析、バリデーション
- **filters.go**: フィルタリングロジック、正規表現処理
- **diff.go**: 差分分析アルゴリズム、比較ロジック
- **output.go**: 出力形式変換、ファイル書き込み

#### Medium Priority（重要）
- **logger.go**: ログレベル制御、フォーマット
- **progress.go**: プログレス計算、ETA算出
- **types.go**: 構造体バリデーション

#### Low Priority（補助）
- **main.go**: CLI引数パース（複雑な統合テストのため）
- **clients.go**: OCI SDK依存（モック困難）
- **discovery.go**: OCI SDK依存（モック困難）

### 3. モジュール別テスト設計

#### config_test.go
```go
// テスト対象関数
- LoadConfig()
- getDefaultConfig()
- ValidateConfig()
- GenerateDefaultConfigFile()
- MergeWithCLIArgs()

// テストケース
- デフォルト設定の正確性
- YAML形式の正しい解析
- 無効な設定値の適切なエラーハンドリング
- ファイル存在有無での動作確認
- 設定優先度の正確性（CLI > config file > default）
```

#### filters_test.go
```go
// テスト対象関数
- ValidateFilterConfig()
- CompileFilters()
- ApplyCompartmentFilter()
- ApplyResourceTypeFilter()
- ApplyNameFilter()
- ParseResourceTypeList()
- ParseCompartmentList()

// テストケース
- 正規表現パターンの正確な動作
- 不正なOCID形式の検出
- コンパートメントフィルタの正確な適用
- リソースタイプ別名の正確な変換
- 空フィルタでの全通し動作
- 複合フィルタの正確な動作
```

#### diff_test.go
```go
// テスト対象関数
- CompareDumps()
- LoadResourcesFromFile()
- CreateResourceMap()
- FindAddedResources()
- FindRemovedResources()
- FindModifiedResources()
- CompareResourceDetails()
- BuildDiffResult()
- OutputDiffResult()

// テストケース
- 同一ファイル比較での変更なし検出
- 追加リソースの正確な検出
- 削除リソースの正確な検出
- 変更リソースの詳細差分検出
- 統計情報の正確な計算
- JSON/Text出力形式の正確性
- 無効なJSONファイルでのエラーハンドリング
```

#### output_test.go
```go
// テスト対象関数
- outputJSON()
- outputCSV()
- outputTSV()
- outputToFile()
- ValidateOutputFormat()

// テストケース
- JSON形式の正確な出力
- CSV形式の正確な出力（エスケープ含む）
- TSV形式の正確な出力
- ファイル出力の正確性
- stdout出力の正確性
- 無効な出力形式でのエラーハンドリング
- 書き込み権限なしでのエラーハンドリング
```

#### logger_test.go
```go
// テスト対象関数
- NewLogger()
- ParseLogLevel()
- Info(), Verbose(), Debug(), Error()

// テストケース
- 各ログレベルでの適切な出力制御
- 無効なログレベルでのエラーハンドリング
- ログフォーマットの正確性
- 同期安全性の確認
```

#### progress_test.go
```go
// テスト対象関数
- NewProgressTracker()
- Update()
- CalculateETA()
- FormatDuration()

// テストケース
- プログレス計算の正確性
- ETA算出の妥当性
- 異常値入力での安定性
- 同期安全性の確認
```

#### types_test.go
```go
// テスト対象関数
- ResourceInfo構造体のバリデーション
- Config構造体のバリデーション

// テストケース
- 必須フィールドの検証
- データ型の整合性確認
- JSON serialization/deserialization
```

## テスト実装方針

### 1. テストファイル命名規則
```
{module}_test.go
例: config_test.go, filters_test.go
```

### 2. テスト関数命名規則
```go
func Test{FunctionName}_{Scenario}(t *testing.T)
例: TestLoadConfig_ValidFile(t *testing.T)
    TestApplyNameFilter_RegexMatch(t *testing.T)
```

### 3. Table-Driven Test例
```go
func TestParseResourceTypeList(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected []string
    }{
        {"empty string", "", nil},
        {"single type", "compute_instances", []string{"compute_instances"}},
        {"multiple types", "compute_instances,vcns", []string{"compute_instances", "vcns"}},
        {"with spaces", " compute_instances , vcns ", []string{"compute_instances", "vcns"}},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ParseResourceTypeList(tt.input)
            if !reflect.DeepEqual(result, tt.expected) {
                t.Errorf("ParseResourceTypeList(%q) = %v, want %v", tt.input, result, tt.expected)
            }
        })
    }
}
```

### 4. モック設計

#### ファイルシステムモック
```go
type MockFileSystem struct {
    files map[string][]byte
    errors map[string]error
}

func (m *MockFileSystem) ReadFile(filename string) ([]byte, error) {
    if err, exists := m.errors[filename]; exists {
        return nil, err
    }
    return m.files[filename], nil
}
```

#### JSON出力モック
```go
type MockWriter struct {
    buffer bytes.Buffer
    errors map[string]error
}

func (m *MockWriter) Write(p []byte) (int, error) {
    return m.buffer.Write(p)
}
```

## テストデータ設計

### 1. サンプル設定ファイル
```yaml
# test_config_valid.yaml
version: "1.0"
general:
  timeout: 300
  log_level: "debug"
  output_format: "json"
  progress: true

# test_config_invalid.yaml
version: "invalid"
general:
  timeout: "invalid"
  log_level: "invalid"
```

### 2. サンプルリソースデータ
```json
// test_resources.json
[
  {
    "resource_type": "ComputeInstance",
    "resource_name": "test-instance-01",
    "ocid": "ocid1.instance.oc1..test001",
    "compartment_id": "ocid1.compartment.oc1..test",
    "additional_info": {
      "shape": "VM.Standard2.1",
      "primary_ip": "10.0.1.10"
    }
  }
]
```

### 3. 差分テストデータ
```json
// old_resources.json, new_resources.json
// 追加、削除、変更の各パターンを含む
```

## カバレッジ測定

### 1. カバレッジコマンド
```bash
# カバレッジ付きテスト実行
go test -coverprofile=coverage.out ./...

# カバレッジレポート生成
go tool cover -html=coverage.out -o coverage.html

# カバレッジ率表示
go tool cover -func=coverage.out
```

### 2. カバレッジ目標
- **Overall**: 80%以上
- **Critical modules** (config, filters, diff): 90%以上
- **Utility modules** (logger, progress): 70%以上

## CI/CD統合

### 1. GitHub Actions設定
```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: 1.19
    - run: go test -v -coverprofile=coverage.out ./...
    - run: go tool cover -func=coverage.out
```

### 2. テスト自動化
```bash
#!/bin/bash
# run_tests.sh
set -e

echo "Running unit tests..."
go test -v -coverprofile=coverage.out ./...

echo "Generating coverage report..."
go tool cover -html=coverage.out -o coverage.html

echo "Coverage summary:"
go tool cover -func=coverage.out

echo "Tests completed successfully!"
```

## ベンチマーク計画

### 1. 性能テスト対象
- **diff.go**: 大量データでの差分分析性能
- **filters.go**: 複雑な正規表現フィルタ性能
- **output.go**: 大量リソース出力性能

### 2. ベンチマーク例
```go
func BenchmarkCompareDumps(b *testing.B) {
    oldResources := generateTestResources(1000)
    newResources := generateTestResources(1000)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        CompareDumps("old.json", "new.json", DiffConfig{})
    }
}
```

## 制限事項

### 1. 現在の制限
- **OCI SDK依存**: clients.go, discovery.goの完全テストは困難
- **外部依存**: ファイルシステム、ネットワークアクセス
- **並行処理**: 一部の同期処理テストの複雑性

### 2. 将来改善
- **統合テスト**: Docker環境でのOCIモック
- **E2Eテスト**: 完全なワークフローテスト
- **負荷テスト**: 大規模データでの性能評価

## まとめ

Phase 2D ユニットテスト実装により：

1. **品質保証**: 各モジュールの動作信頼性向上
2. **回帰防止**: 将来の変更における品質維持
3. **ドキュメント**: テストコードによる仕様明確化
4. **CI/CD**: 自動化された継続的品質管理

80%以上のテストカバレッジ達成により、OCI Resource Dump CLIツールの企業利用における信頼性が大幅に向上する。