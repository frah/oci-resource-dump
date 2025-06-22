#!/bin/bash

# 精密タイムアウトテストスイート
# OCI Resource Dump CLIのタイムアウト動作を正確に測定・検証

set -e

# テスト設定
BINARY="./oci-resource-dump"
TEST_RESULTS_DIR="test_results"
LOG_FILE="$TEST_RESULTS_DIR/timeout_test.log"
DETAILED_LOG="$TEST_RESULTS_DIR/detailed_measurements.log"

# 色付き出力用
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 結果集計用変数
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 初期化
setup_test_environment() {
    echo -e "${BLUE}=== 精密タイムアウトテストスイート開始 ===${NC}"
    
    # テスト結果ディレクトリ作成
    mkdir -p "$TEST_RESULTS_DIR"
    
    # ログファイル初期化
    echo "=== OCI Resource Dump タイムアウトテスト ===" > "$LOG_FILE"
    echo "開始時刻: $(date)" >> "$LOG_FILE"
    echo "" >> "$LOG_FILE"
    
    echo "=== 詳細測定ログ ===" > "$DETAILED_LOG"
    echo "開始時刻: $(date)" >> "$DETAILED_LOG"
    echo "" >> "$DETAILED_LOG"
    
    # バイナリ存在確認
    if [[ ! -f "$BINARY" ]]; then
        echo -e "${RED}エラー: $BINARY が見つかりません${NC}"
        echo "go build -o oci-resource-dump *.go を実行してください"
        exit 1
    fi
    
    echo -e "${GREEN}テスト環境準備完了${NC}"
    echo ""
}

# 精密時間測定関数
measure_execution_time() {
    local timeout_seconds=$1
    local test_name=$2
    local tolerance_percent=${3:-20}  # デフォルト許容誤差20%
    
    echo -e "${YELLOW}テスト実行: $test_name (期待: ${timeout_seconds}秒)${NC}"
    
    # 実行時間測定
    local start_time=$(date +%s.%N)
    
    # タイムアウト付きでコマンド実行
    timeout $((timeout_seconds + 30)) $BINARY --timeout $timeout_seconds --log-level silent >/dev/null 2>&1 || true
    
    local end_time=$(date +%s.%N)
    
    # 実行時間計算（小数点以下2桁）
    local actual_time=$(echo "$end_time - $start_time" | bc -l)
    local actual_time_rounded=$(printf "%.2f" $actual_time)
    
    # 許容範囲計算
    local tolerance=$(echo "$timeout_seconds * $tolerance_percent / 100" | bc -l)
    local min_time=$(echo "$timeout_seconds - $tolerance" | bc -l)
    local max_time=$(echo "$timeout_seconds + $tolerance" | bc -l)
    
    # 結果判定
    local result="FAIL"
    if (( $(echo "$actual_time >= $min_time" | bc -l) )) && (( $(echo "$actual_time <= $max_time" | bc -l) )); then
        result="PASS"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        echo -e "${GREEN}✅ $result${NC}: 期待=${timeout_seconds}s, 実際=${actual_time_rounded}s (許容範囲: ${min_time}-${max_time}s)"
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo -e "${RED}❌ $result${NC}: 期待=${timeout_seconds}s, 実際=${actual_time_rounded}s (許容範囲: ${min_time}-${max_time}s)"
    fi
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    # ログ記録
    echo "$test_name: 期待=${timeout_seconds}s, 実際=${actual_time_rounded}s, 結果=$result" >> "$LOG_FILE"
    echo "$(date '+%H:%M:%S.%3N') - $test_name - 開始:$start_time, 終了:$end_time, 実行時間:$actual_time_rounded, 判定:$result" >> "$DETAILED_LOG"
    
    echo ""
    sleep 1  # テスト間のインターバル
}

# 段階別タイムアウトテスト
test_stage_specific_timeouts() {
    echo -e "${BLUE}=== 段階別タイムアウトテスト ===${NC}"
    
    # 超短時間テスト（認証・初期化段階）
    measure_execution_time 1 "超短時間テスト(1秒)" 50
    measure_execution_time 2 "短時間テスト(2秒)" 30
    measure_execution_time 3 "短時間テスト(3秒)" 30
    
    # 中期間テスト（クライアント初期化段階）
    measure_execution_time 5 "中期間テスト(5秒)" 20
    measure_execution_time 10 "中期間テスト(10秒)" 15
    
    # 長期間テスト（リソース発見段階）
    measure_execution_time 30 "長期間テスト(30秒)" 10
}

# ログレベル別テスト
test_with_different_log_levels() {
    echo -e "${BLUE}=== ログレベル別タイムアウトテスト ===${NC}"
    
    local timeout_val=5
    
    # 各ログレベルでの実行時間測定
    for log_level in "silent" "normal" "verbose" "debug"; do
        echo -e "${YELLOW}ログレベル: $log_level${NC}"
        
        local start_time=$(date +%s.%N)
        timeout $((timeout_val + 10)) $BINARY --timeout $timeout_val --log-level $log_level >/dev/null 2>&1 || true
        local end_time=$(date +%s.%N)
        
        local actual_time=$(echo "$end_time - $start_time" | bc -l)
        local actual_time_rounded=$(printf "%.2f" $actual_time)
        
        echo "  実行時間: ${actual_time_rounded}秒"
        echo "ログレベル $log_level: ${actual_time_rounded}秒" >> "$LOG_FILE"
        
        sleep 1
    done
    echo ""
}

# 連続実行安定性テスト
test_consecutive_runs() {
    echo -e "${BLUE}=== 連続実行安定性テスト ===${NC}"
    
    local timeout_val=3
    local runs=5
    local times=()
    
    echo "同一タイムアウト値(${timeout_val}秒)で${runs}回連続実行..."
    
    for ((i=1; i<=runs; i++)); do
        echo -n "  実行 $i/$runs: "
        
        local start_time=$(date +%s.%N)
        timeout $((timeout_val + 10)) $BINARY --timeout $timeout_val --log-level silent >/dev/null 2>&1 || true
        local end_time=$(date +%s.%N)
        
        local actual_time=$(echo "$end_time - $start_time" | bc -l)
        local actual_time_rounded=$(printf "%.2f" $actual_time)
        times+=($actual_time_rounded)
        
        echo "${actual_time_rounded}秒"
        sleep 1
    done
    
    # 統計計算
    local sum=0
    local min=${times[0]}
    local max=${times[0]}
    
    for time in "${times[@]}"; do
        sum=$(echo "$sum + $time" | bc -l)
        if (( $(echo "$time < $min" | bc -l) )); then
            min=$time
        fi
        if (( $(echo "$time > $max" | bc -l) )); then
            max=$time
        fi
    done
    
    local avg=$(echo "scale=2; $sum / $runs" | bc -l)
    local variance=0
    for time in "${times[@]}"; do
        local diff=$(echo "$time - $avg" | bc -l)
        variance=$(echo "$variance + ($diff * $diff)" | bc -l)
    done
    local std_dev=$(echo "scale=2; sqrt($variance / $runs)" | bc -l)
    
    echo -e "${GREEN}統計結果:${NC}"
    echo "  平均: ${avg}秒"
    echo "  最小: ${min}秒"
    echo "  最大: ${max}秒"
    echo "  標準偏差: ${std_dev}秒"
    
    # ログ記録
    echo "連続実行統計 - 平均:${avg}s, 最小:${min}s, 最大:${max}s, 標準偏差:${std_dev}s" >> "$LOG_FILE"
    echo ""
}

# プロセス終了確認テスト
test_process_termination() {
    echo -e "${BLUE}=== プロセス終了確認テスト ===${NC}"
    
    local timeout_val=2
    echo "プロセスの完全終了を確認中..."
    
    # バックグラウンドで実行し、プロセス追跡
    $BINARY --timeout $timeout_val --log-level silent >/dev/null 2>&1 &
    local pid=$!
    
    echo "  プロセスID: $pid"
    
    # プロセス監視
    local elapsed=0
    while kill -0 $pid 2>/dev/null; do
        sleep 0.1
        elapsed=$(echo "$elapsed + 0.1" | bc -l)
        
        # 異常に長い場合は強制終了
        if (( $(echo "$elapsed > $((timeout_val + 5))" | bc -l) )); then
            echo -e "${RED}  警告: プロセスが${elapsed}秒経過後も終了していません（強制終了）${NC}"
            kill -9 $pid 2>/dev/null || true
            break
        fi
    done
    
    local final_elapsed=$(printf "%.1f" $elapsed)
    echo "  プロセス終了確認: ${final_elapsed}秒"
    
    if (( $(echo "$elapsed <= $((timeout_val + 1))" | bc -l) )); then
        echo -e "${GREEN}✅ プロセス終了: 正常${NC}"
    else
        echo -e "${RED}❌ プロセス終了: 遅延あり${NC}"
    fi
    
    echo "プロセス終了時間: ${final_elapsed}秒" >> "$LOG_FILE"
    echo ""
}

# 結果サマリー表示
show_test_summary() {
    echo -e "${BLUE}=== テスト結果サマリー ===${NC}"
    echo "実行テスト数: $TOTAL_TESTS"
    echo -e "成功: ${GREEN}$PASSED_TESTS${NC}"
    echo -e "失敗: ${RED}$FAILED_TESTS${NC}"
    
    if [[ $FAILED_TESTS -eq 0 ]]; then
        echo -e "${GREEN}🎉 全テスト成功！${NC}"
        echo "全テスト成功" >> "$LOG_FILE"
    else
        echo -e "${RED}⚠️  一部テスト失敗${NC}"
        echo "一部テスト失敗 ($FAILED_TESTS/$TOTAL_TESTS)" >> "$LOG_FILE"
    fi
    
    echo ""
    echo "詳細ログ: $LOG_FILE"
    echo "測定データ: $DETAILED_LOG"
    echo "完了時刻: $(date)" >> "$LOG_FILE"
}

# メイン実行
main() {
    setup_test_environment
    
    # 基本コンパイル確認
    echo -e "${BLUE}=== バイナリ動作確認 ===${NC}"
    if $BINARY --help >/dev/null 2>&1; then
        echo -e "${GREEN}✅ バイナリ動作確認: 正常${NC}"
    else
        echo -e "${RED}❌ バイナリ動作確認: 失敗${NC}"
        exit 1
    fi
    echo ""
    
    # 各テスト実行
    test_stage_specific_timeouts
    test_with_different_log_levels
    test_consecutive_runs
    test_process_termination
    
    show_test_summary
}

# bcコマンド確認
if ! command -v bc &> /dev/null; then
    echo -e "${RED}エラー: bcコマンドが必要です${NC}"
    echo "インストール: sudo apt-get install bc"
    exit 1
fi

# メイン実行
main "$@"