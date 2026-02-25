#!/usr/bin/env bash
set -euo pipefail

###############################################################################
# setup-github-secrets.sh - Create IAM access keys and store as GitHub secrets
#
# Usage: ./infrastructure/setup-github-secrets.sh
#
# Prerequisites:
#   - AWS CLI authenticated
#   - gh CLI authenticated (gh auth login)
#   - infrastructure/config.sh exists (run deploy-infra.sh first)
###############################################################################

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/config.sh"

if [ ! -f "$CONFIG_FILE" ]; then
  echo "ERROR: ${CONFIG_FILE} not found."
  echo "Run ./infrastructure/deploy-infra.sh first."
  exit 1
fi

source "$CONFIG_FILE"

echo "============================================"
echo "  Runbook - GitHub Secrets Setup"
echo "============================================"
echo ""
echo "IAM User: ${DEPLOY_USER_NAME}"
echo ""

# Check for existing keys
echo "Checking existing access keys..."
EXISTING_KEYS=$(aws iam list-access-keys \
  --user-name "${DEPLOY_USER_NAME}" \
  --query "AccessKeyMetadata[].AccessKeyId" \
  --output text)

KEY_COUNT=$(echo "$EXISTING_KEYS" | tr -s '\t' '\n' | grep -c . || true)

if [ "$KEY_COUNT" -ge 2 ]; then
  # Delete the oldest key (first in list)
  OLDEST_KEY=$(echo "$EXISTING_KEYS" | awk '{print $1}')
  echo "Maximum keys reached. Deleting oldest key: ${OLDEST_KEY}"
  aws iam delete-access-key \
    --user-name "${DEPLOY_USER_NAME}" \
    --access-key-id "${OLDEST_KEY}"
  echo "Deleted."
  echo ""
fi

# Create new access key
echo "Creating new access key..."
KEY_OUTPUT=$(aws iam create-access-key \
  --user-name "${DEPLOY_USER_NAME}" \
  --output json)

ACCESS_KEY_ID=$(echo "$KEY_OUTPUT" | python3 -c "import sys,json; print(json.load(sys.stdin)['AccessKey']['AccessKeyId'])")
SECRET_ACCESS_KEY=$(echo "$KEY_OUTPUT" | python3 -c "import sys,json; print(json.load(sys.stdin)['AccessKey']['SecretAccessKey'])")

echo "Access key created: ${ACCESS_KEY_ID}"
echo ""

# Store as GitHub secrets
echo "Storing credentials as GitHub secrets..."
gh secret set AWS_ACCESS_KEY_ID --body "${ACCESS_KEY_ID}"
echo "  -> AWS_ACCESS_KEY_ID set"

gh secret set AWS_SECRET_ACCESS_KEY --body "${SECRET_ACCESS_KEY}"
echo "  -> AWS_SECRET_ACCESS_KEY set"

echo ""
echo "============================================"
echo "  GitHub secrets configured!"
echo "============================================"
echo ""
echo "Secrets set:"
echo "  AWS_ACCESS_KEY_ID"
echo "  AWS_SECRET_ACCESS_KEY"
echo ""
echo "Non-secret config (committed to repo):"
echo "  S3_BUCKET=${S3_BUCKET}"
echo "  CLOUDFRONT_DISTRIBUTION_ID=${CLOUDFRONT_DISTRIBUTION_ID}"
echo "  ARTIFACTS_URL=${ARTIFACTS_URL}"
