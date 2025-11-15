#!/bin/bash

# Generate Postman collection from OpenAPI spec
# Requires: openapi2postmanv2 (install: npm install -g openapi-to-postmanv2)

API_URL="http://localhost:8080"
SWAGGER_JSON_URL="${API_URL}/swagger/doc.json"
OUTPUT_FILE="i18n-center-api.postman_collection.json"

echo "📥 Fetching OpenAPI spec from ${SWAGGER_JSON_URL}..."
curl -s "${SWAGGER_JSON_URL}" > /tmp/swagger.json

if [ ! -f /tmp/swagger.json ]; then
    echo "❌ Failed to fetch Swagger JSON"
    exit 1
fi

echo "🔄 Converting to Postman collection..."

# Check if openapi2postmanv2 is installed
if ! command -v openapi2postmanv2 &> /dev/null; then
    echo "⚠️  openapi2postmanv2 not found. Installing..."
    npm install -g openapi-to-postmanv2
fi

openapi2postmanv2 -s /tmp/swagger.json -o "${OUTPUT_FILE}"

if [ -f "${OUTPUT_FILE}" ]; then
    echo "✅ Postman collection generated: ${OUTPUT_FILE}"
    echo "📦 Import this file into Postman to use the API"
else
    echo "❌ Failed to generate Postman collection"
    exit 1
fi

