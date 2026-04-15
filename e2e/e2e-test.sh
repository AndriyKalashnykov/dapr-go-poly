#!/usr/bin/env bash
#
# End-to-end test suite for dapr-go-poly.
#
# Assumes `make e2e` has brought up the full stack via the compose overlay:
#   - product-service on :1000
#   - order-service   on :1001 (with RabbitMQ consumer backed by Postgres)
#   - postgres, rabbitmq
#
# Covers the e2e requirements from the test-coverage-analysis skill:
#   1. Service health (all endpoints reachable)
#   2. Data round-trip (POST product, GET product, PUT, DELETE)
#   3. Compose routing (requests hit exposed ports, reach services, real DB)
#   4. Negative case (validation rejects bad input; 404 on unknown id)
#   5. Async pipeline (RabbitMQ publish → consumer → Postgres → GET /api/orders)
#
set -euo pipefail

PRODUCT_BASE="${PRODUCT_BASE:-http://localhost:1000}"
ORDER_BASE="${ORDER_BASE:-http://localhost:1001}"
RABBIT_API="${RABBIT_API:-http://localhost:15672}"
RABBIT_USER="${RABBIT_USER:-guest}"
RABBIT_PASS="${RABBIT_PASS:-guest}"
READINESS_TIMEOUT="${READINESS_TIMEOUT:-60}"

PASS=0
FAIL=0

color_green=$(printf '\033[32m')
color_red=$(printf '\033[31m')
color_reset=$(printf '\033[0m')

log_pass() {
  echo "${color_green}PASS${color_reset}: $*"
  PASS=$((PASS + 1))
}

log_fail() {
  echo "${color_red}FAIL${color_reset}: $*"
  FAIL=$((FAIL + 1))
}

wait_for_service() {
  local url="$1" name="$2"
  local deadline=$((SECONDS + READINESS_TIMEOUT))

  while (( SECONDS < deadline )); do
    if curl -sf -o /dev/null -m 2 "$url"; then
      echo "  ${color_green}${name} ready${color_reset} after $((SECONDS))s"
      return 0
    fi
    sleep 1
  done

  echo "${color_red}FATAL${color_reset}: ${name} did not become ready within ${READINESS_TIMEOUT}s (polling $url)"
  return 1
}

assert_status() {
  local method="$1" url="$2" expected="$3" body="${4:-}"
  local opts=(-s -o /dev/null -w '%{http_code}' -X "$method")
  if [[ -n "$body" ]]; then
    opts+=(-H 'Content-Type: application/json' -d "$body")
  fi

  local status
  status=$(curl "${opts[@]}" "$url" || echo "000")

  if [[ "$status" == "$expected" ]]; then
    log_pass "$method $url → $status"
  else
    log_fail "$method $url → $status (expected $expected)"
  fi
}

assert_json_field() {
  local url="$1" field="$2" expected="$3"
  local value
  value=$(curl -sf "$url" | jq -r ".$field // empty" 2>/dev/null || echo "")

  if [[ "$value" == "$expected" ]]; then
    log_pass "GET $url .$field == \"$expected\""
  else
    log_fail "GET $url .$field == \"$value\" (expected \"$expected\")"
  fi
}

# Publish a JSON message to the given RabbitMQ queue via the management HTTP
# API. Posting to `amq.default` with the queue name as routing key is
# equivalent to publishing on the default exchange. Avoids bundling
# rabbitmqadmin or a language client in the test image.
publish_to_queue() {
  local queue="$1" payload="$2"
  local url="${RABBIT_API}/api/exchanges/%2F/amq.default/publish"
  local envelope
  envelope=$(jq -cn --arg rk "$queue" --arg p "$payload" \
    '{properties: {delivery_mode: 2}, routing_key: $rk, payload: $p, payload_encoding: "string"}')

  local response
  response=$(curl -s -u "${RABBIT_USER}:${RABBIT_PASS}" -X POST \
    -H 'Content-Type: application/json' \
    -d "$envelope" \
    "$url")

  # RabbitMQ returns {"routed": true} on success.
  if echo "$response" | jq -e '.routed == true' >/dev/null 2>&1; then
    return 0
  fi
  echo "  publish response: $response" >&2
  return 1
}

echo
echo "=== Waiting for services (timeout: ${READINESS_TIMEOUT}s each) ==="
wait_for_service "$PRODUCT_BASE/api/products" "product-service" || exit 1
wait_for_service "$ORDER_BASE/api/orders"     "order-service"   || exit 1
wait_for_auth_service() {
  local url="$1" name="$2" user="$3" pass="$4"
  local deadline=$((SECONDS + READINESS_TIMEOUT))

  while (( SECONDS < deadline )); do
    if curl -sf -o /dev/null -u "${user}:${pass}" -m 2 "$url"; then
      echo "  ${color_green}${name} ready${color_reset} after $((SECONDS))s"
      return 0
    fi
    sleep 1
  done

  echo "${color_red}FATAL${color_reset}: ${name} did not become ready within ${READINESS_TIMEOUT}s (polling $url)"
  return 1
}

wait_for_auth_service "$RABBIT_API/api/overview" "rabbitmq-mgmt" "$RABBIT_USER" "$RABBIT_PASS" || exit 1

# The 'orders' queue is declared by OrdersConsumer.ExecuteAsync on startup,
# so must wait for it to exist before attempting to publish; otherwise the
# management API returns {"routed": false}.
wait_for_auth_service "$RABBIT_API/api/queues/%2F/orders" "orders-queue (declared by consumer)" "$RABBIT_USER" "$RABBIT_PASS" || exit 1

echo
echo "=== E2E Tests ==="
echo "    product-service: $PRODUCT_BASE"
echo "    order-service:   $ORDER_BASE"
echo "    rabbitmq mgmt:   $RABBIT_API"
echo

# ------------------------------------------------------------------
# Layer 1 — service health: every service responds on its public port
# ------------------------------------------------------------------
echo "--- Health ---"
assert_status GET "$PRODUCT_BASE/api/products" "200"
assert_status GET "$ORDER_BASE/api/orders"     "200"

# ------------------------------------------------------------------
# Layer 2 — product-service CRUD round-trip through real Postgres
# ------------------------------------------------------------------
echo
echo "--- product-service CRUD ---"

create_payload='{"name":"E2E Widget","description":"Created by e2e-test.sh","price":19.99}'
response=$(curl -sf -X POST -H 'Content-Type: application/json' \
  -d "$create_payload" "$PRODUCT_BASE/api/products")
product_id=$(echo "$response" | jq -r '.id')

if [[ -z "$product_id" || "$product_id" == "null" ]]; then
  log_fail "POST /api/products did not return id. Body: $response"
else
  log_pass "POST /api/products → id=$product_id"
fi

assert_json_field "$PRODUCT_BASE/api/products/$product_id" "name" "E2E Widget"

update_payload='{"name":"E2E Widget Pro","description":"Updated","price":24.50}'
assert_status PUT "$PRODUCT_BASE/api/products/$product_id" "204" "$update_payload"
assert_json_field "$PRODUCT_BASE/api/products/$product_id" "name" "E2E Widget Pro"

# ------------------------------------------------------------------
# Layer 3 — product-service validation negative cases
# ------------------------------------------------------------------
echo
echo "--- product-service validation ---"

assert_status POST "$PRODUCT_BASE/api/products" "400" '{"name":"","description":"bad","price":1.00}'
assert_status POST "$PRODUCT_BASE/api/products" "400" '{"name":"ok","description":"ok","price":-1.00}'
assert_status GET  "$PRODUCT_BASE/api/products/00000000-0000-0000-0000-000000000000" "404"

# ------------------------------------------------------------------
# Layer 4 — order-service reachability + Postgres wiring
# ------------------------------------------------------------------
echo
echo "--- order-service ---"

orders=$(curl -sf "$ORDER_BASE/api/orders")
if echo "$orders" | jq -e 'type == "array"' >/dev/null 2>&1; then
  log_pass "GET $ORDER_BASE/api/orders returned JSON array"
else
  log_fail "GET $ORDER_BASE/api/orders did not return a JSON array. Body: $orders"
fi

# ------------------------------------------------------------------
# Layer 5 — async pipeline: RabbitMQ → OrdersConsumer → Postgres
# ------------------------------------------------------------------
echo
echo "--- async order pipeline ---"

order_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
order_payload=$(jq -cn --arg id "$order_id" --arg pid "$product_id" \
  '{Id: $id, ProductId: $pid, Quantity: 7, Price: 42.00}')

if publish_to_queue "orders" "$order_payload"; then
  log_pass "Published order $order_id to RabbitMQ 'orders' queue"
else
  log_fail "Failed to publish to RabbitMQ"
fi

deadline=$((SECONDS + 30))
matched=0
while (( SECONDS < deadline )); do
  if curl -sf "$ORDER_BASE/api/orders" \
      | jq -e --arg id "$order_id" 'any(.[]; (.id // .Id) == $id)' >/dev/null 2>&1; then
    matched=1
    break
  fi
  sleep 1
done

if (( matched == 1 )); then
  log_pass "Order $order_id persisted via RabbitMQ → OrdersConsumer → Postgres"
else
  log_fail "Order $order_id not visible in GET /api/orders within 30s"
fi

# ------------------------------------------------------------------
# Cleanup
# ------------------------------------------------------------------
echo
echo "--- Cleanup ---"
assert_status DELETE "$PRODUCT_BASE/api/products/$product_id" "204"
assert_status GET    "$PRODUCT_BASE/api/products/$product_id" "404"

echo
echo "=== Results: ${color_green}${PASS} passed${color_reset}, ${color_red}${FAIL} failed${color_reset} ==="

if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
