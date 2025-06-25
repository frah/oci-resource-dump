package main

import (
	"testing"
)

// 基本的なユニットテスト
func TestBasicOperations(t *testing.T) {
	// テストが実行できることを確認
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
}

func TestMain(m *testing.M) {
	// テストの前後処理
	m.Run()
}
