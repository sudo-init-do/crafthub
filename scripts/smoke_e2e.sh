#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "Starting Docker stack (postgres, redis, api)" >&2
docker compose up -d postgres redis api

echo "Waiting for API health..." >&2
for i in {1..60}; do
  if curl -s http://localhost:8080/health | grep -q "ok"; then echo "API up" >&2; break; fi
  sleep 1
  if [ "$i" -eq 60 ]; then echo "API did not become healthy in time" >&2; exit 1; fi
done

echo "Signing up users..." >&2
BUYER_TOKEN=$(curl -s -X POST http://localhost:8080/signup \
  -H "Content-Type: application/json" \
  -d '{"name":"Buyer One","email":"buyer1@example.com","password":"secret123"}' \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("token",""))' || true)
SELLER_TOKEN=$(curl -s -X POST http://localhost:8080/signup \
  -H "Content-Type: application/json" \
  -d '{"name":"Seller One","email":"seller1@example.com","password":"secret123"}' \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("token",""))' || true)

if [ -z "$BUYER_TOKEN" ]; then
  echo "Signup for buyer failed; attempting login..." >&2
  BUYER_TOKEN=$(curl -s -X POST http://localhost:8080/login \
    -H "Content-Type: application/json" \
    -d '{"email":"buyer1@example.com","password":"secret123"}' \
    | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("token",""))' || true)
fi

if [ -z "$SELLER_TOKEN" ]; then
  echo "Signup for seller failed; attempting login..." >&2
  SELLER_TOKEN=$(curl -s -X POST http://localhost:8080/login \
    -H "Content-Type: application/json" \
    -d '{"email":"seller1@example.com","password":"secret123"}' \
    | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("token",""))' || true)
fi

if [ -z "$BUYER_TOKEN" ] || [ -z "$SELLER_TOKEN" ]; then
  echo "Auth failed; tokens missing" >&2
  exit 1
fi
echo "Tokens acquired (buyer_len=${#BUYER_TOKEN} seller_len=${#SELLER_TOKEN})" >&2

echo "Funding buyer wallet..." >&2
TOPUP_RESP=$(curl -s -X POST http://localhost:8080/wallet/topups/init \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount":500000}')
TOPUP_STATUS=$(printf "%s" "$TOPUP_RESP" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("status",""))')
TOPUP_ID=$(printf "%s" "$TOPUP_RESP" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("topup_id",""))')
if [ "$TOPUP_STATUS" != "completed" ] && [ -n "$TOPUP_ID" ]; then
  curl -s -X POST "http://localhost:8080/wallet/topups/$TOPUP_ID/confirm" \
    -H "Authorization: Bearer $BUYER_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"topup_id\":\"$TOPUP_ID\",\"status\":\"success\"}" >/dev/null
fi
echo "Buyer balance:" >&2
curl -s http://localhost:8080/wallet/balance -H "Authorization: Bearer $BUYER_TOKEN" | tee /dev/stderr

echo "Creating service as seller..." >&2
SERVICE_RAW=$(curl -s -X POST http://localhost:8080/marketplace/services \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Logo Design","description":"Pro logo","price":50000,"category":"design","delivery_time_days":3}')
SERVICE_ID=$(printf "%s" "$SERVICE_RAW" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("service_id",""))' || true)
if [ -z "$SERVICE_ID" ]; then echo "Service creation failed; resp=$SERVICE_RAW" >&2; exit 1; fi
echo "Service: $SERVICE_ID" >&2

echo "Creating order as buyer..." >&2
ORDER_RAW=$(curl -s -X POST http://localhost:8080/marketplace/orders \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"service_id\":\"$SERVICE_ID\"}")
ORDER_ID=$(printf "%s" "$ORDER_RAW" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("order_id",""))' || true)
if [ -z "$ORDER_ID" ]; then echo "Order creation failed; resp=$ORDER_RAW" >&2; exit 1; fi
echo "Order: $ORDER_ID" >&2

echo "Buyer sends message..." >&2
MSG_ID=$(curl -s -X POST "http://localhost:8080/marketplace/orders/$ORDER_ID/messages" \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"Hi, excited to work together!"}' \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("message_id",""))')
if [ -z "$MSG_ID" ]; then echo "Send message failed" >&2; exit 1; fi
echo "Message: $MSG_ID" >&2

echo "Seller unread count (expect 1):" >&2
curl -s "http://localhost:8080/marketplace/orders/$ORDER_ID/messages/unread_count" -H "Authorization: Bearer $SELLER_TOKEN" | tee /dev/stderr

echo "Seller marks message read..." >&2
curl -s -X POST "http://localhost:8080/marketplace/orders/$ORDER_ID/messages/$MSG_ID/read" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -H "Content-Type: application/json" -d '{}' | tee /dev/stderr

echo "Seller unread count (expect 0):" >&2
curl -s "http://localhost:8080/marketplace/orders/$ORDER_ID/messages/unread_count" -H "Authorization: Bearer $SELLER_TOKEN" | tee /dev/stderr

echo "Seller accepts order..." >&2
curl -s -X POST "http://localhost:8080/marketplace/orders/$ORDER_ID/accept" -H "Authorization: Bearer $SELLER_TOKEN" -H "Content-Type: application/json" -d '{}' | tee /dev/stderr

echo "Seller delivers order..." >&2
curl -s -X POST "http://localhost:8080/marketplace/orders/$ORDER_ID/deliver" -H "Authorization: Bearer $SELLER_TOKEN" -H "Content-Type: application/json" -d '{}' | tee /dev/stderr

echo "Buyer completes order..." >&2
curl -s -X POST "http://localhost:8080/marketplace/orders/$ORDER_ID/complete" -H "Authorization: Bearer $BUYER_TOKEN" -H "Content-Type: application/json" -d '{}' | tee /dev/stderr

echo "Buyer notifications:" >&2
curl -s "http://localhost:8080/notifications" -H "Authorization: Bearer $BUYER_TOKEN" | tee /dev/stderr
echo "Seller notifications:" >&2
curl -s "http://localhost:8080/notifications" -H "Authorization: Bearer $SELLER_TOKEN" | tee /dev/stderr

echo "Smoke test finished: order=$ORDER_ID service=$SERVICE_ID message=$MSG_ID" >&2
