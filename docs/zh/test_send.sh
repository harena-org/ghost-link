#!/bin/bash
set -e

GHOSTLINK="./build/ghostlink"
RPC_URL="https://solana-devnet.gateway.tatum.io"
INBOX_NAME="test-$(date +%s)"

echo "=== 1. 创建收件箱: $INBOX_NAME ==="
echo "" | $GHOSTLINK inbox create "$INBOX_NAME"

echo ""
echo "=== 2. 获取收件箱地址 ==="
INBOX_ADDR=$($GHOSTLINK inbox list 2>/dev/null | grep -A1 "名称: $INBOX_NAME" | grep "地址:" | awk '{print $2}')
echo "收件箱地址: $INBOX_ADDR"

echo ""
echo "=== 3. 设置为默认收件箱 ==="
$GHOSTLINK inbox set-default "$INBOX_NAME"

echo ""
echo "=== 4. 发送测试消息 ==="
$GHOSTLINK send -u "$RPC_URL" --to "$INBOX_ADDR" -m "Hello GhostLink! 这是一条测试消息 $(date)"

echo ""
echo "=== 5. 等待链上确认 (10秒) ==="
sleep 20

echo ""
echo "=== 6. 接收消息 ==="
$GHOSTLINK receive -u "$RPC_URL"

echo ""
echo "=== 测试完成 ==="
