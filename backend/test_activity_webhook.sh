#!/bin/bash

# Configuration
URL="http://localhost:8090/api/webhooks/activity"
SECRET="${ACTIVITY_WEBHOOK_SECRET}"

# Test payload
PAYLOAD='{"type":"test_activity","title":"Test","contact_id":"","source_app":"test","source_id":"test123","source_url":"","metadata":{},"occurred_at":"2026-01-24T10:00:00Z"}'

echo "=========================================="
echo "Testing Activity Webhook HMAC Validation"
echo "=========================================="
echo ""

# Test 1: No signature (should fail with 401)
echo "Test 1: No signature (should fail with 401)"
echo "-------------------------------------------"
curl -X POST "$URL" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" \
  -w "\nHTTP Status: %{http_code}\n"
echo ""
echo ""

# Test 2: Invalid signature (should fail with 401)
echo "Test 2: Invalid signature (should fail with 401)"
echo "--------------------------------------------------"
curl -X POST "$URL" \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: invalid_signature" \
  -d "$PAYLOAD" \
  -w "\nHTTP Status: %{http_code}\n"
echo ""
echo ""

# Test 3: Valid signature (should succeed with 200)
echo "Test 3: Valid signature (should succeed with 200)"
echo "---------------------------------------------------"
if [ -z "$SECRET" ]; then
    echo "ERROR: ACTIVITY_WEBHOOK_SECRET environment variable not set"
    echo "Please set it with: export ACTIVITY_WEBHOOK_SECRET=your_secret"
else
    SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')
    echo "Computed signature: $SIGNATURE"
    curl -X POST "$URL" \
      -H "Content-Type: application/json" \
      -H "X-Webhook-Signature: $SIGNATURE" \
      -d "$PAYLOAD" \
      -w "\nHTTP Status: %{http_code}\n"
fi
echo ""
echo ""

echo "=========================================="
echo "Testing complete"
echo "=========================================="
