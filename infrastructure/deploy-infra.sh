#!/usr/bin/env bash
set -euo pipefail

###############################################################################
# deploy-infra.sh - Deploy CloudFormation stack for runbookmcp.dev infrastructure
#
# Usage: ./infrastructure/deploy-infra.sh [--env ENV] [--cert-arn ARN]
#
# Flags:
#   --env ENV       Environment name (default: production)
#   --cert-arn ARN  ACM certificate ARN in us-east-1 (optional - auto-created
#                   if not provided and no existing cert is found)
#
# Prerequisites:
#   - AWS CLI authenticated
#   - runbookmcp.dev hosted zone in Route53 (for automatic DNS validation)
#
# Writes infrastructure/config.sh after successful deploy.
###############################################################################

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/config.sh"
TEMPLATE_FILE="${SCRIPT_DIR}/cloudformation.yaml"
REGION="us-west-2"
CERT_REGION="us-east-1"
DOMAIN="runbookmcp.dev"

ENV="production"
CERT_ARN=""

while [ $# -gt 0 ]; do
  case "$1" in
    --env)      ENV="$2";      shift 2 ;;
    --cert-arn) CERT_ARN="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

STACK_NAME="runbook-${ENV}"

echo "============================================"
echo "  Runbook - Infrastructure Deploy"
echo "============================================"
echo ""
echo "Stack     : ${STACK_NAME}"
echo "Region    : ${REGION}"
echo "Env       : ${ENV}"
echo ""

# ---- Certificate ------------------------------------------------------------

ensure_certificate() {
  echo "Checking for ACM certificate..."

  # Use provided ARN if given
  if [ -n "$CERT_ARN" ]; then
    echo "  Using provided certificate: ${CERT_ARN}"
    return 0
  fi

  # Look for an already-issued cert
  CERT_ARN=$(aws acm list-certificates \
    --region "$CERT_REGION" \
    --certificate-statuses ISSUED \
    --query "CertificateSummaryList[?DomainName=='${DOMAIN}'].CertificateArn" \
    --output text 2>/dev/null | awk '{print $1}')

  if [ -n "$CERT_ARN" ]; then
    echo "  Found issued certificate: ${CERT_ARN}"
    return 0
  fi

  # Look for a pending cert (previously requested, not yet validated)
  CERT_ARN=$(aws acm list-certificates \
    --region "$CERT_REGION" \
    --certificate-statuses PENDING_VALIDATION \
    --query "CertificateSummaryList[?DomainName=='${DOMAIN}'].CertificateArn" \
    --output text 2>/dev/null | awk '{print $1}')

  if [ -z "$CERT_ARN" ]; then
    echo "  No certificate found. Requesting one..."
    CERT_ARN=$(aws acm request-certificate \
      --region "$CERT_REGION" \
      --domain-name "$DOMAIN" \
      --validation-method DNS \
      --query 'CertificateArn' \
      --output text)
    echo "  Requested: ${CERT_ARN}"
    sleep 5  # Give ACM a moment to prepare validation records
  else
    echo "  Found pending certificate: ${CERT_ARN}"
  fi

  # Wait for validation records to appear
  echo "  Waiting for DNS validation records..."
  local attempts=0
  local all_ready=false
  while [ "$all_ready" = false ]; do
    if [ $attempts -ge 12 ]; then
      echo "ERROR: Timed out waiting for ACM validation records."
      exit 1
    fi
    pending=$(aws acm describe-certificate \
      --region "$CERT_REGION" \
      --certificate-arn "$CERT_ARN" \
      --query 'Certificate.DomainValidationOptions[?ResourceRecord.Name==null].DomainName' \
      --output text 2>/dev/null || true)
    if [ -z "$pending" ]; then
      all_ready=true
    else
      attempts=$((attempts + 1))
      sleep 5
    fi
  done

  # Try to validate via Route53 automatically
  local zone_id
  zone_id=$(aws route53 list-hosted-zones \
    --query "HostedZones[?Name=='${DOMAIN}.'].Id" \
    --output text | sed 's|/hostedzone/||')

  if [ -n "$zone_id" ]; then
    echo "  Found Route53 hosted zone (${zone_id}). Adding validation records..."
    # Build change batch for all unique validation CNAMEs
    changes=$(aws acm describe-certificate \
      --region "$CERT_REGION" \
      --certificate-arn "$CERT_ARN" \
      --query 'Certificate.DomainValidationOptions[].ResourceRecord' \
      --output json | python3 -c "
import json, sys
records = json.load(sys.stdin)
seen = set()
changes = []
for r in records:
    if r['Name'] not in seen:
        seen.add(r['Name'])
        changes.append({'Action':'UPSERT','ResourceRecordSet':{'Name':r['Name'],'Type':'CNAME','TTL':300,'ResourceRecords':[{'Value':r['Value']}]}})
print(json.dumps({'Changes': changes}))
")
    aws route53 change-resource-record-sets \
      --hosted-zone-id "$zone_id" \
      --change-batch "$changes" \
      --query 'ChangeInfo.Status' \
      --output text > /dev/null
    echo "  Waiting for certificate validation (takes a few minutes)..."
    aws acm wait certificate-validated \
      --region "$CERT_REGION" \
      --certificate-arn "$CERT_ARN"
    echo "  Certificate issued: ${CERT_ARN}"
  else
    echo ""
    echo "  ERROR: '${DOMAIN}' is not managed by Route53 in this account."
    echo "  Add DNS CNAME records to validate the certificate, then re-run:"
    aws acm describe-certificate \
      --region "$CERT_REGION" \
      --certificate-arn "$CERT_ARN" \
      --query 'Certificate.DomainValidationOptions[].ResourceRecord' \
      --output table
    echo ""
    echo "  Certificate ARN (use --cert-arn once validated):"
    echo "    ${CERT_ARN}"
    exit 1
  fi
}

ensure_certificate
echo ""

# ---- Look up hosted zone -----------------------------------------------------

ZONE_ID=$(aws route53 list-hosted-zones \
  --query "HostedZones[?Name=='${DOMAIN}.'].Id" \
  --output text | sed 's|/hostedzone/||')

if [ -z "$ZONE_ID" ]; then
  echo "ERROR: No Route53 hosted zone found for ${DOMAIN}"
  exit 1
fi

echo "Hosted zone: ${ZONE_ID}"
echo ""

# ---- Deploy CloudFormation --------------------------------------------------

echo "Deploying CloudFormation stack..."
aws cloudformation deploy \
  --region "${REGION}" \
  --stack-name "${STACK_NAME}" \
  --template-file "${TEMPLATE_FILE}" \
  --parameter-overrides \
    "Environment=${ENV}" \
    "AcmCertificateArn=${CERT_ARN}" \
    "HostedZoneId=${ZONE_ID}" \
  --capabilities CAPABILITY_NAMED_IAM \
  --no-fail-on-empty-changeset

echo "Stack deployed successfully."
echo ""

# ---- Retrieve outputs -------------------------------------------------------

echo "Retrieving outputs..."
BUCKET_NAME=$(aws cloudformation describe-stacks \
  --region "${REGION}" \
  --stack-name "${STACK_NAME}" \
  --query "Stacks[0].Outputs[?OutputKey=='BucketName'].OutputValue" \
  --output text)

DISTRIBUTION_ID=$(aws cloudformation describe-stacks \
  --region "${REGION}" \
  --stack-name "${STACK_NAME}" \
  --query "Stacks[0].Outputs[?OutputKey=='CloudFrontDistributionId'].OutputValue" \
  --output text)

DEPLOY_USER=$(aws cloudformation describe-stacks \
  --region "${REGION}" \
  --stack-name "${STACK_NAME}" \
  --query "Stacks[0].Outputs[?OutputKey=='DeployUserName'].OutputValue" \
  --output text)

echo "  Bucket             : ${BUCKET_NAME}"
echo "  Distribution ID    : ${DISTRIBUTION_ID}"
echo "  Deploy User        : ${DEPLOY_USER}"
echo ""

# ---- Write config -----------------------------------------------------------

echo "Writing config to '${CONFIG_FILE}'..."
cat > "${CONFIG_FILE}" <<EOF
# Auto-generated by deploy-infra.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Source this file to set infrastructure environment variables:
#   source infrastructure/config.sh

export S3_BUCKET="${BUCKET_NAME}"
export CLOUDFRONT_DISTRIBUTION_ID="${DISTRIBUTION_ID}"
export ARTIFACTS_URL="https://runbookmcp.dev"
export DEPLOY_USER_NAME="${DEPLOY_USER}"
EOF

echo "Config written. Source it with:"
echo "  source infrastructure/config.sh"
echo ""
echo "============================================"
echo "  Deployment complete!"
echo "============================================"
