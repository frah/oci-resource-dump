# Phase 2C: 簡易差分分析機能設計書

## 概要
OCI Resource Dump CLIツールに2つのJSON出力ファイル間の差分分析機能を追加。
リソース変更の追跡とレポート生成により、インフラ管理の監査とトラブルシューティングを支援。

## 設計方針

### 1. シンプル設計
- **JSON比較のみ**: 複雑な履歴管理は実装しない
- **ファイルベース**: 2つのJSONファイルを比較
- **即座分析**: リアルタイム処理、永続化なし

### 2. 差分検出タイプ
```
- Added: 新規追加されたリソース
- Removed: 削除されたリソース  
- Modified: 変更されたリソース（詳細情報の変更）
- Unchanged: 変更なし（オプション出力）
```

### 3. 比較キー戦略
```
Primary Key: OCID (Oracle Cloud Identifier)
Secondary Key: CompartmentID + ResourceType + ResourceName
```

## CLI インターフェース設計

### 基本コマンド
```bash
# 基本差分分析
./oci-resource-dump --compare-files old.json new.json

# 差分出力ファイル指定
./oci-resource-dump --compare-files old.json new.json --diff-output diff_report.json

# テキスト形式レポート
./oci-resource-dump --compare-files old.json new.json --diff-format text --diff-output report.txt

# 詳細モード（Unchangedも含む）
./oci-resource-dump --compare-files old.json new.json --diff-detailed
```

### CLI引数仕様
```go
var compareFiles *string = flag.String("compare-files", "", "Comma-separated pair of JSON files to compare (old,new)")
var diffOutput *string = flag.String("diff-output", "", "Output file for diff analysis (default: stdout)")
var diffFormat *string = flag.String("diff-format", "json", "Diff output format: json, text")
var diffDetailed *bool = flag.Bool("diff-detailed", false, "Include unchanged resources in diff output")
```

## 設定ファイル統合

### YAML設定拡張
```yaml
version: "1.0"
general:
    timeout: 300
    log_level: normal
    output_format: json
    progress: true
output:
    file: ""
filters:
    include_compartments: []
    exclude_compartments: []
    include_resource_types: []
    exclude_resource_types: []
    name_pattern: ""
    exclude_name_pattern: ""
diff:                        # 新規追加
    format: "json"           # json, text
    detailed: false          # include unchanged resources
    output_file: ""          # output file path
```

### 設定構造体
```go
type DiffConfig struct {
    Format     string `yaml:"format"`      // "json" or "text"
    Detailed   bool   `yaml:"detailed"`    // include unchanged resources
    OutputFile string `yaml:"output_file"` // output file path
}

type AppConfig struct {
    Version string        `yaml:"version"`
    General GeneralConfig `yaml:"general"`
    Output  OutputConfig  `yaml:"output"`
    Filters FilterConfig  `yaml:"filters"`
    Diff    DiffConfig    `yaml:"diff"`  // 新規追加
}
```

## 差分分析アルゴリズム

### 1. データ構造設計
```go
// 差分分析結果
type DiffResult struct {
    Summary      DiffSummary      `json:"summary"`
    Added        []ResourceInfo   `json:"added"`
    Removed      []ResourceInfo   `json:"removed"`
    Modified     []ModifiedResource `json:"modified"`
    Unchanged    []ResourceInfo   `json:"unchanged,omitempty"`
    Timestamp    string          `json:"timestamp"`
    OldFile      string          `json:"old_file"`
    NewFile      string          `json:"new_file"`
}

// 差分サマリ
type DiffSummary struct {
    TotalOld      int            `json:"total_old"`
    TotalNew      int            `json:"total_new"`
    Added         int            `json:"added"`
    Removed       int            `json:"removed"`
    Modified      int            `json:"modified"`
    Unchanged     int            `json:"unchanged"`
    ByResourceType map[string]DiffStats `json:"by_resource_type"`
}

// リソースタイプ別統計
type DiffStats struct {
    Added     int `json:"added"`
    Removed   int `json:"removed"`
    Modified  int `json:"modified"`
    Unchanged int `json:"unchanged"`
}

// 変更されたリソース詳細
type ModifiedResource struct {
    ResourceInfo ResourceInfo               `json:"resource_info"`
    Changes      []FieldChange             `json:"changes"`
}

// フィールド変更詳細
type FieldChange struct {
    Field    string      `json:"field"`
    OldValue interface{} `json:"old_value"`
    NewValue interface{} `json:"new_value"`
}
```

### 2. 比較アルゴリズム
```go
func CompareDumps(oldFile, newFile string) (*DiffResult, error) {
    // 1. JSONファイル読み込み
    oldResources := loadResourcesFromFile(oldFile)
    newResources := loadResourcesFromFile(newFile)
    
    // 2. OCIDベースマップ作成
    oldMap := createResourceMap(oldResources)
    newMap := createResourceMap(newResources)
    
    // 3. 差分検出
    added := findAddedResources(oldMap, newMap)
    removed := findRemovedResources(oldMap, newMap)
    modified := findModifiedResources(oldMap, newMap)
    unchanged := findUnchangedResources(oldMap, newMap)
    
    // 4. 結果構築
    return buildDiffResult(added, removed, modified, unchanged, oldFile, newFile)
}
```

### 3. 変更検出ロジック
```go
func findModifiedResources(oldMap, newMap map[string]ResourceInfo) []ModifiedResource {
    var modified []ModifiedResource
    
    for ocid, oldResource := range oldMap {
        if newResource, exists := newMap[ocid]; exists {
            changes := compareResourceDetails(oldResource, newResource)
            if len(changes) > 0 {
                modified = append(modified, ModifiedResource{
                    ResourceInfo: newResource,
                    Changes:      changes,
                })
            }
        }
    }
    return modified
}

func compareResourceDetails(old, new ResourceInfo) []FieldChange {
    var changes []FieldChange
    
    // 基本フィールド比較
    if old.ResourceName != new.ResourceName {
        changes = append(changes, FieldChange{
            Field:    "ResourceName",
            OldValue: old.ResourceName,
            NewValue: new.ResourceName,
        })
    }
    
    // AdditionalInfo詳細比較
    oldInfo := old.AdditionalInfo
    newInfo := new.AdditionalInfo
    
    // 全キーを統合
    allKeys := getAllKeys(oldInfo, newInfo)
    for _, key := range allKeys {
        oldVal, oldExists := oldInfo[key]
        newVal, newExists := newInfo[key]
        
        if !oldExists && newExists {
            changes = append(changes, FieldChange{
                Field:    fmt.Sprintf("AdditionalInfo.%s", key),
                OldValue: nil,
                NewValue: newVal,
            })
        } else if oldExists && !newExists {
            changes = append(changes, FieldChange{
                Field:    fmt.Sprintf("AdditionalInfo.%s", key),
                OldValue: oldVal,
                NewValue: nil,
            })
        } else if oldExists && newExists && !reflect.DeepEqual(oldVal, newVal) {
            changes = append(changes, FieldChange{
                Field:    fmt.Sprintf("AdditionalInfo.%s", key),
                OldValue: oldVal,
                NewValue: newVal,
            })
        }
    }
    
    return changes
}
```

## 出力形式設計

### 1. JSON形式（詳細）
```json
{
  "summary": {
    "total_old": 150,
    "total_new": 148,
    "added": 5,
    "removed": 7,
    "modified": 12,
    "unchanged": 131,
    "by_resource_type": {
      "ComputeInstance": {
        "added": 2,
        "removed": 1,
        "modified": 3,
        "unchanged": 45
      }
    }
  },
  "added": [
    {
      "resource_type": "ComputeInstance",
      "resource_name": "web-server-03",
      "ocid": "ocid1.instance.oc1..newinstance",
      "compartment_id": "ocid1.compartment.oc1..prod",
      "additional_info": {
        "primary_ip": "10.0.1.15",
        "shape": "VM.Standard.E4.Flex"
      }
    }
  ],
  "removed": [
    {
      "resource_type": "ComputeInstance", 
      "resource_name": "old-server-01",
      "ocid": "ocid1.instance.oc1..removedinstance",
      "compartment_id": "ocid1.compartment.oc1..prod",
      "additional_info": {
        "primary_ip": "10.0.1.10",
        "shape": "VM.Standard2.1"
      }
    }
  ],
  "modified": [
    {
      "resource_info": {
        "resource_type": "ComputeInstance",
        "resource_name": "web-server-01",
        "ocid": "ocid1.instance.oc1..changedinstance",
        "compartment_id": "ocid1.compartment.oc1..prod",
        "additional_info": {
          "primary_ip": "10.0.1.12",
          "shape": "VM.Standard.E4.Flex"
        }
      },
      "changes": [
        {
          "field": "AdditionalInfo.shape",
          "old_value": "VM.Standard2.1",
          "new_value": "VM.Standard.E4.Flex"
        },
        {
          "field": "AdditionalInfo.primary_ip",
          "old_value": "10.0.1.11",
          "new_value": "10.0.1.12"
        }
      ]
    }
  ],
  "timestamp": "2025-06-23T02:15:30Z",
  "old_file": "dump_2025-06-20.json",
  "new_file": "dump_2025-06-23.json"
}
```

### 2. テキスト形式（サマリ重視）
```
OCI Resource Dump Comparison Report
===================================

Files Compared:
  Old: dump_2025-06-20.json (150 resources)
  New: dump_2025-06-23.json (148 resources)
  
Generated: 2025-06-23T02:15:30Z

SUMMARY
-------
Total Changes: 24 resources affected
  Added:     5 resources
  Removed:   7 resources  
  Modified: 12 resources
  Unchanged: 131 resources

CHANGES BY RESOURCE TYPE
------------------------
ComputeInstance: +2, -1, ~3 (51 total)
VCN:             +0, -2, ~1 (25 total)
Subnet:          +1, -3, ~2 (42 total)
BlockVolume:     +2, -1, ~6 (30 total)

ADDED RESOURCES (5)
-------------------
+ ComputeInstance: web-server-03 (ocid1.instance.oc1..newinstance)
  Compartment: ocid1.compartment.oc1..prod
  Shape: VM.Standard.E4.Flex, IP: 10.0.1.15

+ ComputeInstance: web-server-04 (ocid1.instance.oc1..newinstance2)
  Compartment: ocid1.compartment.oc1..prod
  Shape: VM.Standard.E4.Flex, IP: 10.0.1.16

REMOVED RESOURCES (7)
---------------------
- ComputeInstance: old-server-01 (ocid1.instance.oc1..removedinstance)
  Compartment: ocid1.compartment.oc1..prod
  Shape: VM.Standard2.1, IP: 10.0.1.10

- VCN: legacy-network (ocid1.vcn.oc1..removedvcn)
  Compartment: ocid1.compartment.oc1..dev
  CIDR: 172.16.0.0/16

MODIFIED RESOURCES (12)
-----------------------
~ ComputeInstance: web-server-01 (ocid1.instance.oc1..changedinstance)
  Compartment: ocid1.compartment.oc1..prod
  Changes:
    - shape: VM.Standard2.1 → VM.Standard.E4.Flex
    - primary_ip: 10.0.1.11 → 10.0.1.12

~ BlockVolume: data-volume-01 (ocid1.volume.oc1..changedvolume)
  Compartment: ocid1.compartment.oc1..prod  
  Changes:
    - size_gb: 100 → 200
    - performance_tier: Balanced → Higher Performance
```

## 実装ファイル構成

### diff.go (新規作成)
```go
// 主要機能
- CompareDumps(oldFile, newFile string, config DiffConfig) (*DiffResult, error)
- LoadResourcesFromFile(filename string) ([]ResourceInfo, error)
- CreateResourceMap(resources []ResourceInfo) map[string]ResourceInfo
- FindAddedResources(oldMap, newMap map[string]ResourceInfo) []ResourceInfo
- FindRemovedResources(oldMap, newMap map[string]ResourceInfo) []ResourceInfo
- FindModifiedResources(oldMap, newMap map[string]ResourceInfo) []ModifiedResource
- CompareResourceDetails(old, new ResourceInfo) []FieldChange
- BuildDiffResult(...) *DiffResult
- OutputDiffResult(result *DiffResult, config DiffConfig) error
- OutputDiffText(result *DiffResult, writer io.Writer) error
- OutputDiffJSON(result *DiffResult, writer io.Writer) error
```

## エラーハンドリング

### 1. ファイル関連エラー
```go
// ファイル存在チェック
if !fileExists(oldFile) {
    return nil, fmt.Errorf("old file not found: %s", oldFile)
}

// JSON解析エラー
if err := json.Unmarshal(data, &resources); err != nil {
    return nil, fmt.Errorf("failed to parse JSON file %s: %w", filename, err)
}
```

### 2. 比較制限
```go
// 大量データ警告
if len(oldResources) > 10000 || len(newResources) > 10000 {
    logger.Verbose("Warning: Large dataset detected (%d, %d resources). Analysis may take time.", 
                   len(oldResources), len(newResources))
}
```

## 性能考慮

### 1. メモリ効率
- **ストリーミング読み込み**: 大容量JSONファイル対応
- **マップベース比較**: O(n)時間計算量
- **段階的処理**: メモリ使用量制御

### 2. 処理最適化
```go
// リソースマップ最適化
func createResourceMap(resources []ResourceInfo) map[string]ResourceInfo {
    resourceMap := make(map[string]ResourceInfo, len(resources))
    for _, resource := range resources {
        // OCIDを主キーとして使用（一意性保証）
        if resource.OCID != "" {
            resourceMap[resource.OCID] = resource
        }
    }
    return resourceMap
}
```

## 使用例とワークフロー

### 1. 定期監査ワークフロー
```bash
# 1. 現在のリソースダンプ取得
./oci-resource-dump --output-file current_$(date +%Y%m%d).json

# 2. 前回との差分分析
./oci-resource-dump --compare-files last_week.json current_$(date +%Y%m%d).json \
  --diff-format text --diff-output weekly_changes.txt

# 3. 詳細JSON分析（必要時）
./oci-resource-dump --compare-files last_week.json current_$(date +%Y%m%d).json \
  --diff-detailed --diff-output detailed_analysis.json
```

### 2. デプロイメント検証
```bash
# デプロイ前
./oci-resource-dump --output-file pre_deploy.json

# デプロイ後  
./oci-resource-dump --output-file post_deploy.json

# 変更確認
./oci-resource-dump --compare-files pre_deploy.json post_deploy.json \
  --diff-format text
```

### 3. 設定ファイル使用
```yaml
# diff_config.yaml
diff:
  format: "text"
  detailed: false
  output_file: "change_report.txt"
```

```bash
./oci-resource-dump --compare-files old.json new.json
# diff設定が自動適用される
```

## 制限事項と拡張計画

### 現在の制限
- **履歴管理なし**: 2ファイル間比較のみ
- **JSONフォーマット限定**: CSV/TSV比較は非対応
- **静的分析**: リアルタイム変更追跡なし

### 将来拡張可能性
- **時系列分析**: 複数ポイント間の変更履歴
- **アラート機能**: 重要変更の自動通知
- **フィルタリング**: 差分結果に対するフィルタ適用
- **レポート生成**: HTML/PDFレポート出力

## まとめ

Phase 2C 簡易差分分析機能により：

1. **変更追跡**: インフラ変更の可視化
2. **監査支援**: 定期的な差分レポート
3. **トラブルシューティング**: 問題発生時の変更点特定
4. **デプロイ検証**: 計画通りの変更実施確認

企業環境でのOCIインフラ管理において、変更管理とガバナンス強化に貢献する重要機能として位置づけられます。