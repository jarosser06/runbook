#!/usr/bin/env bash
set -euo pipefail

###############################################################################
# deploy.sh - Build and deploy runbook artifacts to S3 + CloudFront
#
# Usage: ./deploy.sh [flags]
#
# Flags:
#   --infra           Deploy CloudFormation infrastructure first
#   --clean           Remove build/ directory before building
#   --cert-arn ARN    Override ACM certificate ARN (optional; auto-created otherwise)
#
# This script:
#   1. Optionally deploys infrastructure
#   2. Sources infrastructure/config.sh for S3/CloudFront settings
#   3. Runs scripts/build-all.sh to cross-compile
#   4. Copies website/ and install scripts into build/
#   5. Uploads build/ to S3 via aws s3 sync
#   6. Creates CloudFront invalidation
###############################################################################

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

DEPLOY_INFRA=false
CLEAN_BUILD=false
CERT_ARN=""

while [ $# -gt 0 ]; do
  case "$1" in
    --infra) DEPLOY_INFRA=true; shift ;;
    --clean) CLEAN_BUILD=true; shift ;;
    --cert-arn) CERT_ARN="$2"; shift 2 ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

echo "============================================"
echo "  Runbook - Build & Deploy"
echo "============================================"
echo ""

# ---- Optionally deploy infrastructure ---------------------------------------
if [ "$DEPLOY_INFRA" = true ]; then
  echo "Deploying infrastructure..."
  INFRA_ARGS=()
  if [ -n "$CERT_ARN" ]; then
    INFRA_ARGS+=(--cert-arn "$CERT_ARN")
  fi
  "${SCRIPT_DIR}/infrastructure/deploy-infra.sh" "${INFRA_ARGS[@]}"
  echo ""
fi

# ---- Source config -----------------------------------------------------------
CONFIG_FILE="${SCRIPT_DIR}/infrastructure/config.sh"
if [ ! -f "$CONFIG_FILE" ]; then
  echo "ERROR: ${CONFIG_FILE} not found."
  echo "Run ./infrastructure/deploy-infra.sh first, or use --infra flag."
  exit 1
fi

source "$CONFIG_FILE"
echo "S3 Bucket       : ${S3_BUCKET}"
echo "Distribution ID : ${CLOUDFRONT_DISTRIBUTION_ID}"
echo "Artifacts URL   : ${ARTIFACTS_URL}"
echo ""

# ---- Optionally clean -------------------------------------------------------
if [ "$CLEAN_BUILD" = true ]; then
  echo "Cleaning build directory..."
  rm -rf "${SCRIPT_DIR}/build"
  echo ""
fi

# ---- Build all platforms -----------------------------------------------------
echo "Building all platforms..."
"${SCRIPT_DIR}/scripts/build-all.sh"
echo ""

# ---- Copy website and install scripts into build/ ---------------------------
echo "Copying website files to build/..."
if [ -d "${SCRIPT_DIR}/website" ]; then
  cp -r "${SCRIPT_DIR}/website/." "${SCRIPT_DIR}/build/"
  echo "  -> website/"
fi

echo "Copying install scripts to build/..."
cp "${SCRIPT_DIR}/scripts/install.sh" "${SCRIPT_DIR}/build/"
cp "${SCRIPT_DIR}/scripts/install.ps1" "${SCRIPT_DIR}/build/"
echo "  -> install.sh"
echo "  -> install.ps1"
echo ""

# ---- Upload to S3 ------------------------------------------------------------
echo "Uploading to S3..."
aws s3 sync "${SCRIPT_DIR}/build/" "s3://${S3_BUCKET}/" \
  --delete \
  --region us-west-2

echo ""

# ---- CloudFront invalidation -------------------------------------------------
echo "Creating CloudFront invalidation..."
aws cloudfront create-invalidation \
  --distribution-id "${CLOUDFRONT_DISTRIBUTION_ID}" \
  --paths "/*" \
  --query 'Invalidation.Status' \
  --output text > /dev/null

echo ""
echo "============================================"
echo "  Deploy complete!"
echo "============================================"
echo ""
echo "Artifacts available at: ${ARTIFACTS_URL}"
echo ""
echo "Install (unix):       curl -fsSL ${ARTIFACTS_URL}/install.sh | bash"
echo "Install (powershell): irm ${ARTIFACTS_URL}/install.ps1 | iex"
echo ""
