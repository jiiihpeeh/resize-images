#!/bin/bash

# Configuration
BASE_URL="http://localhost:8080"
REGISTRATION_SECRET="your-secret-key" # Must match .env
USERNAME="testuser_$(date +%s)" # Unique username
IMAGE_PATH="./cafe.jpg" # Assuming cafe.jpg is in the current directory
OUTPUT_FILE="images.tar.gz"

register_user() {
  echo "--- 1. Registering User ($USERNAME) ---"
  REG_RESPONSE=$(curl -s -X POST "$BASE_URL/auth/register" \
    -H "Content-Type: application/json" \
    -H "X-Registration-Secret: $REGISTRATION_SECRET" \
    -d "{\"username\":\"$USERNAME\"}")

  echo "Response: $REG_RESPONSE"

  # Extract passkey (requires jq, or simple grep/sed)
  PASSKEY=$(echo "$REG_RESPONSE" | grep -o '"passkey":"[^"]*' | cut -d'"' -f4)

  if [ -z "$PASSKEY" ]; then
    echo "Error: Could not extract passkey. Registration might have failed."
    exit 1
  fi
  echo "Got Passkey: $PASSKEY"
  echo ""
}

login_user() {
  echo "--- 2. Logging In ---"
  LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSKEY\"}")

  # Extract token
  TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

  if [ -z "$TOKEN" ]; then
    echo "Error: Could not extract token. Login failed."
    echo "Response: $LOGIN_RESPONSE"
    exit 1
  fi
  echo "Got Token: ${TOKEN:0:10}..."
  echo ""
}

batch_resize() {
  echo "--- 3. Batch Resizing ---"
  # Define tasks JSON
  # This requests:
  # 1. A 800px wide JPEG
  # 2. A 400px wide WebP with 75% quality
  # 3. A 200px wide PNG with a custom prefix
  TASKS='[
    {"width": 800, "format": ["jpg"], "key": "large"},
    {"width": 400, "format": ["webp"], "quality": 75, "key": "medium"},
    {"width": 200, "format": ["png"], "key": "thumb"}
  ]'

  # Make the request
  curl -X POST "$BASE_URL/resize" \
    -H "Authorization: Bearer $TOKEN" \
    -F "image=@$IMAGE_PATH" \
    -F "tasks=$TASKS" \
    -o "$OUTPUT_FILE"

  echo "Download complete: $OUTPUT_FILE"
}

# --- Main execution flow ---

# Ensure the image file exists
if [ ! -f "$IMAGE_PATH" ]; then
  echo "Error: Image file not found at $IMAGE_PATH"
  exit 1
fi

register_user
login_user
batch_resize
