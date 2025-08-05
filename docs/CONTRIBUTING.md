# Contributing to OCI Resource Dump

## 新しいリソースタイプの追加ガイドライン

新しいOCIリソースタイプを追加する際は、以下のガイドラインに従ってください。

### 1. Discovery関数の実装

新しいリソースタイプの discovery 関数を実装する際は、以下の標準パターンを使用してください：

```go
// discoverNewResourceType discovers all new resource types in a compartment
func discoverNewResourceType(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
    var resources []ResourceInfo
    var allResources []oci.NewResourceType

    logger.Debug("Starting new resource type discovery for compartment: %s", compartmentID)

    // Implement pagination to get all resources
    var page *string
    pageCount := 0
    for {
        pageCount++
        logger.Debug("Fetching new resource types page %d for compartment: %s", pageCount, compartmentID)
        req := oci.ListNewResourceTypesRequest{
            CompartmentId: common.String(compartmentID),
            Page:          page,
        }

        resp, err := clients.NewServiceClient.ListNewResourceTypes(ctx, req)

        // CRITICAL: Always handle API errors with proper logging
        if err != nil {
            return nil, fmt.Errorf("failed to list new resource types: %w", err)
        }

        allResources = append(allResources, resp.Items...)

        if resp.OpcNextPage == nil {
            break
        }
        page = resp.OpcNextPage
    }

    // Process resources
    for _, resource := range allResources {
        if resource.LifecycleState != oci.NewResourceTypeLifecycleStateTerminated {
            name := ""
            if resource.DisplayName != nil {
                name = *resource.DisplayName
            }
            ocid := ""
            if resource.Id != nil {
                ocid = *resource.Id
            }

            additionalInfo := make(map[string]interface{})

            // Add resource-specific additional information
            // ...

            resources = append(resources, createResourceInfo(ctx, "NewResourceType", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
        }
    }

    logger.Verbose("Found %d new resource types in compartment %s", len(resources), compartmentID)
    return resources, nil
}
```

### 2. エラーハンドリングの標準

#### 2.1 必須エラーハンドリング

**メインAPI呼び出しのエラー（即座に失敗すべき）:**
```go
resp, err := clients.ServiceClient.ListResources(ctx, req)
if err != nil {
    return nil, fmt.Errorf("failed to list resources: %w", err)
}
```

#### 2.2 オプショナルAPI呼び出しのエラー

**詳細情報取得など、失敗しても処理を継続すべき場合:**
```go
details, err := clients.ServiceClient.GetResourceDetails(ctx, detailReq)
if err != nil {
    logger.Verbose("Error getting resource details for %s: %v", resourceID, err)
    if !isRetriableError(err) {
        logger.Error("Failed to get details for resource %s (compartment %s): %v", resourceID, compartmentID, err)
    }
    // Continue processing without details
} else {
    // Use details
    additionalInfo["detail"] = details.SomeProperty
}
```

#### 2.3 ネストされたリソースのエラー

**ネストされたリソース（例：VmCluster内のDatabase）のエラー:**
```go
nestedResp, err := clients.ServiceClient.ListNestedResources(ctx, nestedReq)
if err != nil {
    logger.Verbose("Error listing nested resources for parent %s: %v", parentID, err)
    if !isRetriableError(err) {
        logger.Error("Failed to discover nested resources for parent %s (compartment %s): %v", parentID, compartmentID, err)
    }
    break // Continue with next parent
}
```

### 3. ログレベルの使い分け

#### 3.1 ログレベル分類

- **`logger.Debug()`**: 詳細なデバッグ情報（ページネーション、API呼び出し詳細）
- **`logger.Verbose()`**: 技術的詳細情報（エラー詳細、処理統計）
- **`logger.Info()`**: 重要な情報（処理進捗、成功統計、ユーザー向け重要情報）
- **`logger.Error()`**: エラー情報（権限不足、重大な失敗）

#### 3.2 ログメッセージ形式

**デバッグログ:**
```go
logger.Debug("Fetching resources page %d for compartment: %s", pageCount, compartmentID)
logger.Debug("Found %d total resources in compartment %s", len(allResources), compartmentID)
```

**エラーログ:**
```go
logger.Verbose("Error getting resource details for %s: %v", resourceID, err)
logger.Error("Failed to discover resources (compartment %s): %v", compartmentID, err)
```

**完了ログ:**
```go
logger.Verbose("Found %d resources in compartment %s", len(resources), compartmentID)
```

### 4. discoverAllResourcesWithProgress関数への登録

新しいリソースタイプを追加したら、`discoverAllResourcesWithProgress()`関数の`discoveryFuncs`マップに登録する：

```go
discoveryFuncs := map[string]func(context.Context, *OCIClients, string) ([]ResourceInfo, error){
    // 既存のリソースタイプ...
    "NewResourceTypes": discoverNewResourceType,  // 新規追加
}
```

### 5. テストの追加

新しいリソースタイプには対応するテストを追加する：

1. **ユニットテスト**: `discovery_test.go`に関数別テスト
2. **統合テスト**: 実際のOCI環境での動作確認
3. **パフォーマンステスト**: 大量データでの性能確認

### 6. ドキュメント更新

以下のドキュメントを更新する：

1. **CLAUDE.md**: サポートリソースタイプの追加
2. **実装ログ**: `docs/implementation/`に詳細実装記録
3. **README.md**: 必要に応じて使用方法の更新

## コードレビューチェックリスト

新しいリソースタイプのプルリクエストでは、以下をチェックする：

### 🔍 必須チェック項目

- [ ] **エラーハンドリング**: 全てのAPI呼び出しで適切なエラーハンドリング
- [ ] **ログ出力**: エラー時に適切なレベルでログ出力
- [ ] **ページネーション**: 大量データに対応するページネーション実装
- [ ] **ライフサイクル状態**: 削除済みリソースの除外
- [ ] **メモリ効率**: 大量リソースでのメモリ使用量考慮
- [ ] **コンテキスト対応**: タイムアウト処理の適切な実装

### 📋 推奨チェック項目

- [ ] **詳細情報**: リソース固有の有用な詳細情報の追加
- [ ] **パフォーマンス**: 効率的なAPI呼び出しパターン
- [ ] **エラー分類**: `isRetriableError()`の適切な使用
- [ ] **統一性**: 既存実装との一貫性
- [ ] **可読性**: コードコメントと関数文書

### ⚠️ 避けるべきパターン

```go
// ❌ BAD: エラーを無視
resp, err := client.ListResources(ctx, req)
// エラーチェックなし

// ❌ BAD: サイレント失敗
if err != nil {
    // ログもエラー返却もなし
    continue
}

// ❌ BAD: 不適切なログレベル
if err != nil {
    logger.Debug("Critical error occurred: %v", err) // Debugは不適切
}

// ❌ BAD: ページネーション未対応
resp, _ := client.ListResources(ctx, req)
// resp.OpcNextPageの処理なし
```

### ✅ 推奨パターン

```go
// ✅ GOOD: 適切なエラーハンドリング
resp, err := client.ListResources(ctx, req)
if err != nil {
    return nil, fmt.Errorf("failed to list resources: %w", err)
}

// ✅ GOOD: オプショナルエラーの適切な処理
details, err := client.GetDetails(ctx, req)
if err != nil {
    logger.Verbose("Error getting details for %s: %v", resourceID, err)
    if !isRetriableError(err) {
        logger.Error("Failed to get details for %s: %v", resourceID, err)
    }
} else {
    additionalInfo["details"] = details
}

// ✅ GOOD: 完全なページネーション
var page *string
for {
    resp, err := client.ListResources(ctx, core.ListResourcesRequest{
        CompartmentId: common.String(compartmentID),
        Page:          page,
    })
    if err != nil {
        return nil, err
    }
    
    allResources = append(allResources, resp.Items...)
    
    if resp.OpcNextPage == nil {
        break
    }
    page = resp.OpcNextPage
}
```

## まとめ

新しいリソースタイプを追加する際は：

1. **標準パターンの使用**: 既存実装との一貫性維持
2. **適切なエラーハンドリング**: 全てのAPI呼び出しでエラー処理
3. **詳細なログ出力**: デバッグと運用の両方に配慮
4. **パフォーマンス考慮**: 大規模環境での動作を想定
5. **テストの追加**: 品質保証のための包括的テスト

このガイドラインに従うことで、堅牢で保守性の高いリソース発見機能を実装できます。