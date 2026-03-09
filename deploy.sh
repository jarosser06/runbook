#!/usr/bin/env bash
set -euo pipefail

###############################################################################
# deploy.sh - Build and deploy runbook
#
# Usage: ./deploy.sh [flags]
#
# Flags:
#   --clean   Remove build/ directory before building
#   --nuke    Destroy all Azure resources
#
# This script:
#   1. Always provisions/updates Azure infrastructure (idempotent)
#   2. Deploys website/ to Azure Static Web App
#   3. If the current commit has a clean semver tag:
#      - Builds binaries for all platforms
#      - Uploads artifacts + install scripts to Azure $web blob container
###############################################################################

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CLEAN_BUILD=false
NUKE=false

for arg in "$@"; do
  case "$arg" in
    --clean) CLEAN_BUILD=true ;;
    --nuke)  NUKE=true ;;
    *) echo "Unknown flag: $arg"; exit 1 ;;
  esac
done

# ---- Nuke -------------------------------------------------------------------
if [ "$NUKE" = true ]; then
  RESOURCE_GROUP="runbook-artifacts-rg"
  echo "============================================"
  echo "  DESTROYING all resources in ${RESOURCE_GROUP}"
  echo "============================================"
  echo ""
  read -r -p "Are you sure? This deletes EVERYTHING. [y/N] " confirm
  if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "Aborted."
    exit 0
  fi
  echo ""
  echo "==> Deleting resource group '${RESOURCE_GROUP}'..."
  az group delete --name "${RESOURCE_GROUP}" --yes --no-wait
  echo "==> Deletion initiated (runs in background)."
  rm -f infrastructure/config.sh
  echo "Done."
  exit 0
fi

# ---- Resolve version from latest semver tag ---------------------------------
VERSION="$(git tag --sort=-version:refname 2>/dev/null | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || echo "dev")"

if [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  DEPLOY_ARTIFACTS=true
else
  DEPLOY_ARTIFACTS=false
fi

echo "============================================"
echo "  Runbook - Deploy"
echo "============================================"
echo ""
echo "Version         : ${VERSION}"
echo ""

# ---- Deploy infrastructure --------------------------------------------------
echo "Deploying infrastructure..."
"${SCRIPT_DIR}/infrastructure/deploy.sh"
echo ""

# ---- Source config ----------------------------------------------------------
CONFIG_FILE="${SCRIPT_DIR}/infrastructure/config.sh"

source "$CONFIG_FILE"
echo "Storage Account : ${STORAGE_ACCOUNT}"
echo "Artifacts URL   : ${ARTIFACTS_URL}"
echo "Website URL     : ${WEBSITE_URL}"
echo ""

# ---- Optionally clean -------------------------------------------------------
if [ "$CLEAN_BUILD" = true ]; then
  echo "Cleaning build directory..."
  rm -rf "${SCRIPT_DIR}/build"
  echo ""
fi

# ---- Check if latest version is already in the bucket ----------------------
if [ "$DEPLOY_ARTIFACTS" = false ] && [ "$VERSION" != "dev" ]; then
  BUCKET_LATEST="$(az storage blob download \
    --account-name "${STORAGE_ACCOUNT}" \
    --container-name "${CONTAINER_NAME}" \
    --name "latest" \
    --file /dev/stdout \
    --auth-mode key \
    --output none \
    2>/dev/null | tr -d '[:space:]' || true)"
  if [ "$BUCKET_LATEST" != "$VERSION" ]; then
    echo "Latest tag ${VERSION} not found in bucket (bucket has: ${BUCKET_LATEST:-none}) — building artifacts."
    DEPLOY_ARTIFACTS=true
  fi
fi

# ---- Build and upload artifacts ---------------------------------------------
if [ "$DEPLOY_ARTIFACTS" = true ]; then
  echo "Building all platforms..."
  VERSION="${VERSION}" "${SCRIPT_DIR}/scripts/build-all.sh"
  echo ""

  echo "Copying install scripts to build/..."
  cp "${SCRIPT_DIR}/scripts/install.sh" "${SCRIPT_DIR}/build/"
  cp "${SCRIPT_DIR}/scripts/install.ps1" "${SCRIPT_DIR}/build/"
  echo "  -> install.sh"
  echo "  -> install.ps1"
  echo ""

  echo "Uploading artifacts to Azure Blob Storage..."
  echo "  Container: ${CONTAINER_NAME}"
  az storage blob upload-batch \
    --account-name "${STORAGE_ACCOUNT}" \
    --destination "${CONTAINER_NAME}" \
    --source "${SCRIPT_DIR}/build" \
    --overwrite \
    --auth-mode key \
    --output none
  echo ""
else
  echo "Skipping artifact build (${VERSION} already in bucket)."
  echo ""
fi

# ---- Deploy website to Azure Static Web App ---------------------------------
if [ -z "${DEPLOYMENT_TOKEN:-}" ]; then
  echo "WARNING: DEPLOYMENT_TOKEN not set in config.sh — skipping website deploy."
  echo "         Run with --infra to provision the Static Web App."
else
  echo "Deploying website to Azure Static Web App..."
  SWA_CLI_DEPLOYMENT_TOKEN="${DEPLOYMENT_TOKEN}" \
    npx --yes @azure/static-web-apps-cli@latest deploy \
    --output-location "${SCRIPT_DIR}/website" \
    --env production
  echo ""
fi

echo "============================================"
echo "  Deploy complete!"
echo "============================================"
echo ""
echo "Artifacts : ${ARTIFACTS_URL}"
echo "Website   : ${WEBSITE_URL}"
echo ""
echo "Install (unix):       curl -fsSL ${ARTIFACTS_URL}install.sh | bash"
echo "Install (powershell): irm ${ARTIFACTS_URL}install.ps1 | iex"
echo ""
