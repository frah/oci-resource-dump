#!/bin/bash

# 詳細タイムアウト分析スクリプト
# 各段階での処理時間を詳細に測定

set -e

BINARY="./oci-resource-dump"
LOG_FILE="detailed_timeout_analysis.log"

# 色付き出力用
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# ログ付きタイムスタンプ測定
measure_with_detailed_logs() {
    local timeout_seconds=$1
    local test_name=$2
    
    echo -e "${BLUE}=== $test_name (タイムアウト: ${timeout_seconds}秒) ===${NC}"
    echo "開始時刻: $(date '+%H:%M:%S.%3N')"
    
    # ログファイル準備
    local detailed_log="test_results/${test_name}_detailed.log"
    mkdir -p test_results
    
    # タイムスタンプ付きで実行
    local start_time=$(date +%s.%N)
    echo "=== $test_name 開始 $(date) ===" > "$detailed_log"
    
    # デバッグレベルで実行し、出力をキャプチャ
    timeout $((timeout_seconds + 10)) $BINARY --timeout $timeout_seconds --log-level debug 2>&1 | while IFS= read -r line; do
        local current_time=$(date +%s.%N)
        local elapsed=$(echo "$current_time - $start_time" | bc -l)
        printf "[%8.3f] %s\n" "$elapsed" "$line" >> "$detailed_log"
    done
    
    local end_time=$(date +%s.%N)
    local total_time=$(echo "$end_time - $start_time" | bc -l)
    
    echo "終了時刻: $(date '+%H:%M:%S.%3N')"
    echo "総実行時間: $(printf "%.3f" $total_time)秒"
    echo ""
    
    # ログ解析
    if [[ -f "$detailed_log" ]]; then
        echo -e "${YELLOW}処理段階分析:${NC}"
        
        # 重要なログメッセージを抽出
        if grep -q "Initializing OCI clients" "$detailed_log"; then
            local init_start=$(grep "Initializing OCI clients" "$detailed_log" | head -1 | awk '{print $1}' | tr -d '[]')
            echo "  クライアント初期化開始: ${init_start}秒"
        fi
        
        if grep -q "OCI clients initialized successfully" "$detailed_log"; then
            local init_end=$(grep "OCI clients initialized successfully" "$detailed_log" | head -1 | awk '{print $1}' | tr -d '[]')
            echo "  クライアント初期化完了: ${init_end}秒"
            
            if [[ -n "$init_start" ]]; then
                local init_duration=$(echo "$init_end - $init_start" | bc -l)
                echo "  初期化所要時間: $(printf "%.3f" $init_duration)秒"
            fi
        fi
        
        if grep -q "Starting resource discovery" "$detailed_log"; then
            local discovery_start=$(grep "Starting resource discovery" "$detailed_log" | head -1 | awk '{print $1}' | tr -d '[]')
            echo "  リソース発見開始: ${discovery_start}秒"
        fi
        
        # エラー検出
        if grep -q -i "error\|timeout\|context deadline exceeded" "$detailed_log"; then
            echo -e "${RED}  エラー/タイムアウト検出${NC}"
            grep -i "error\|timeout\|context deadline exceeded" "$detailed_log" | head -3
        fi
        
        echo "  詳細ログ: $detailed_log"
    fi
    
    echo "---"
    return 0
}

# 単体タイムアウトテスト
test_individual_timeouts() {
    echo -e "${GREEN}=== 単体タイムアウト詳細分析 ===${NC}"
    
    # 各タイムアウト値でテスト
    measure_with_detailed_logs 1 "1秒タイムアウト"
    measure_with_detailed_logs 3 "3秒タイムアウト" 
    measure_with_detailed_logs 5 "5秒タイムアウト"
    measure_with_detailed_logs 10 "10秒タイムアウト"
}

# ネットワーク負荷テスト（OCIサービス呼び出し模擬）
test_network_simulation() {
    echo -e "${GREEN}=== ネットワーク負荷シミュレーション ===${NC}"
    
    # 異なる条件での実行
    for timeout in 2 5 8; do
        echo -e "${YELLOW}タイムアウト ${timeout}秒 - ネットワーク負荷テスト${NC}"
        
        # 複数のプロセスを同時実行してシステム負荷をかける
        for i in {1..3}; do
            $BINARY --timeout $timeout --log-level silent >/dev/null 2>&1 &
            echo "  プロセス $i 開始 (PID: $!)"
        done
        
        # 全プロセス終了まで待機
        wait
        echo "  全プロセス終了"
        sleep 2
    done
}

# 段階的中断テスト
test_stage_interruption() {
    echo -e "${GREEN}=== 段階的中断テスト ===${NC}"
    
    local timeout_val=15
    echo "長時間実行中に段階的に中断..."
    
    # バックグラウンドで実行
    $BINARY --timeout $timeout_val --log-level verbose >/dev/null 2>&1 &
    local pid=$!
    
    echo "プロセス開始 (PID: $pid)"
    
    # 段階的に状態確認
    for i in {1..10}; do
        if ! kill -0 $pid 2>/dev/null; then
            echo "プロセス ${i}秒で終了"
            break
        fi
        echo "  ${i}秒経過: プロセス実行中"
        sleep 1
    done
    
    # 残っていれば強制終了
    if kill -0 $pid 2>/dev/null; then
        echo "プロセス強制終了"
        kill -9 $pid 2>/dev/null || true
    fi
}

# メイン実行
main() {
    echo -e "${BLUE}=== 詳細タイムアウト分析開始 ===${NC}"
    echo "開始時刻: $(date)"
    echo ""
    
    # bcコマンド確認
    if ! command -v bc &> /dev/null; then
        echo -e "${RED}エラー: bcコマンドが必要です${NC}"
        exit 1
    fi
    
    # バイナリ確認
    if [[ ! -f "$BINARY" ]]; then
        echo -e "${RED}エラー: $BINARY が見つかりません${NC}"
        exit 1
    fi
    
    test_individual_timeouts
    test_network_simulation
    test_stage_interruption
    
    echo -e "${BLUE}=== 詳細分析完了 ===${NC}"
    echo "完了時刻: $(date)"
    echo "ログファイル: test_results/*.log"
}

main "$@"