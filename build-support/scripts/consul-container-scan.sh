#!/bin/bash
# local-consul-multiarch-scan.sh
#
# Multi-architecture container image build and security scan script
#
# Usage:
#   ./local-consul-multiarch-scan.sh [PRODUCT] [VERSION]
#
# Arguments:
#   PRODUCT  - Product name (default: consul)
#   VERSION  - Version string (default: local-dev)
#
# Environment Variables:
#   PRODUCT  - Alternative way to set product name
#   VERSION  - Alternative way to set version
#   SCANNER  - Scanner to use: "security-scanner" (default) or "trivy"
#
# Examples:
#   ./local-consul-multiarch-scan.sh
#   ./local-consul-multiarch-scan.sh consul 1.18.0
#   PRODUCT=consul VERSION=1.18.0 ./local-consul-multiarch-scan.sh
#   SCANNER=trivy ./local-consul-multiarch-scan.sh consul 1.18.0
#
set -euo pipefail

# --- Ensure we're running from the project root ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "${PROJECT_ROOT}"

echo "Running from project root: ${PROJECT_ROOT}"
echo ""

# --- Configuration ---
# Accept PRODUCT and VERSION from command line or environment, with defaults
PRODUCT="${1:-${PRODUCT:-consul}}"
VERSION="${2:-${VERSION:-local-dev}}"
OUTPUT_DIR=".bob/artifacts/images"

echo "================================================"
echo "Configuration:"
echo "  PRODUCT: ${PRODUCT}"
echo "  VERSION: ${VERSION}"
echo "  OUTPUT_DIR: ${OUTPUT_DIR}"
echo "================================================"

mkdir -p "${OUTPUT_DIR}"

# --- THE MULTI-ARCH MATRIX ---
DECLARE_IMAGES=(
  "alpine-amd64:./build-support/docker/Consul-Dev.dockerfile::linux/amd64"
  "alpine-arm64:./build-support/docker/Consul-Dev.dockerfile::linux/arm64"
  "ubi-amd64:./Dockerfile:ubi:linux/amd64"
)

SCANNER="${SCANNER:-security-scanner}"

echo ""
echo "🌎 REPLICATING CRT MULTI-ARCH WORKFLOW"
echo "================================================"

# --- Step 1: Ensure Buildx Builder exists ---
if ! docker buildx inspect multi-arch-builder > /dev/null 2>&1; then
    echo "🏗️ Creating multi-arch builder..."
    docker buildx create --name multi-arch-builder --use
    docker buildx inspect --bootstrap
fi

# --- Step 2: Build Shared Binary ---
echo "🔨 Step 1: Building shared Consul binary..."
# Note: In a real multi-arch CRT, you'd build for both GOARCH=amd64 and GOARCH=arm64.
# For local dev, we assume your local binary is compatible or you are testing logic.

make dev

echo "📂 Organizing binaries for Docker build context..."
# FIX: The Consul-Dev.dockerfile expects 'consul' in the ROOT context.
# We copy it to the root so 'COPY consul /bin/consul' works.
cp ./bin/consul ./consul

# We also keep the 'dist' structure for the UBI Dockerfile which uses it
mkdir -p dist/linux/amd64 dist/linux/arm64
cp ./bin/consul ./dist/linux/amd64/consul
cp ./bin/consul ./dist/linux/arm64/consul

# --- Step 3: Build & Scan Loop ---
GLOBAL_REPORT="multiarch-scan-report.txt"
echo "MULTI-ARCH SCAN SUMMARY - $(date)" > "${GLOBAL_REPORT}"

for entry in "${DECLARE_IMAGES[@]}"; do
    IFS=':' read -r FLAVOR DOCKERFILE TARGET PLATFORM <<< "$entry"
    
    IMAGE_TAG="consul:${VERSION}-${FLAVOR}"
    TAR_FILE="${OUTPUT_DIR}/${PRODUCT}_${FLAVOR}.tar"
    
    echo "------------------------------------------------"
    echo "🚀 Processing: ${FLAVOR} (${PLATFORM})"
    
    echo "   📦 Building Image..."
    
    # Build and load the image into Docker
    echo "   📦 Building and loading image into Docker..."
    docker buildx build \
        --platform "${PLATFORM}" \
        -t "${IMAGE_TAG}" \
        -f "${DOCKERFILE}" \
        --build-arg BIN_NAME=consul \
        --build-arg PRODUCT_REVISION="${VERSION}" \
        --build-arg PRODUCT_VERSION="${VERSION}" \
        ${TARGET:+--target "$TARGET"} \
        --load .
    
    echo "   💾 Exporting to Tarball..."
    docker save "${IMAGE_TAG}" -o "${TAR_FILE}"
    
    # 2. Run Scan
    echo "   🔍 Scanning Image..."
    echo "### FLAVOR: ${FLAVOR} ###" >> "${GLOBAL_REPORT}"
    
    SCAN_EXIT_CODE=0
    
    # For UBI images, scan from Docker daemon instead of tarball
    # This allows the scanner to properly detect the Red Hat base OS
    # instead of incorrectly trying to analyze it as Alpine
    if [[ "${FLAVOR}" == *"ubi"* ]]; then
        echo "   Scanning UBI image from Docker daemon (avoids Alpine detection issue)..." | tee -a "${GLOBAL_REPORT}"
        if [ "${SCANNER}" == "security-scanner" ]; then
            SECURITY_SCANNER_CONFIG_FILE=.release/security-scan.hcl \
                ./security-scanner container "docker://${IMAGE_TAG}" >> "${GLOBAL_REPORT}" 2>&1 || SCAN_EXIT_CODE=$?
        else
            trivy image "${IMAGE_TAG}" --severity CRITICAL,HIGH >> "${GLOBAL_REPORT}" 2>&1 || SCAN_EXIT_CODE=$?
        fi
    else
        # For Alpine images, scan from tarball as usual
        if [ "${SCANNER}" == "security-scanner" ]; then
            ./security-scanner container "${TAR_FILE}" >> "${GLOBAL_REPORT}" 2>&1 || SCAN_EXIT_CODE=$?
        else
            trivy image --input "${TAR_FILE}" --severity CRITICAL,HIGH >> "${GLOBAL_REPORT}" 2>&1 || SCAN_EXIT_CODE=$?
        fi
    fi
    
    if [ ${SCAN_EXIT_CODE} -ne 0 ]; then
        echo "   Continuing with remaining images..." >> "${GLOBAL_REPORT}"
    else
        echo "   ✅ Scan completed successfully for ${FLAVOR}"
    fi
    
    # Optional: Clean up the loaded image to save space
    # docker rmi "${IMAGE_TAG}" || true
done

# Cleanup root binary to keep repo clean
rm -f ./consul

echo "================================================"
echo "Done! All artifacts are in ${OUTPUT_DIR}"
echo "Summary report available at: ${GLOBAL_REPORT}"
