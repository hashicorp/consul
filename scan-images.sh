#!/bin/bash
#
# Universal Consul Security Scanner (Binary + Multi-Arch Container)
#
set -euo pipefail

# --- Configuration ---
IMAGES_DIR="bob/artifacts/images"
REPORT_DIR="reports/security-scans"
CONSOLIDATED_REPORT="${REPORT_DIR}/consolidated_scan_report.txt"
SCANNER_BINARY="./security-scanner"
 SCANNER_CONFIG=".release/security-scan.hcl"
TEMP_DIR=$(mktemp -d)

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'

cleanup() { rm -rf "${TEMP_DIR}"; }
trap cleanup EXIT

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   Consul Universal Security Scanner     ${NC}"
echo -e "${BLUE}========================================${NC}"

# --- Pre-flight Checks ---
if [ ! -d "${IMAGES_DIR}" ]; then
    echo -e "${RED}ERROR: Directory ${IMAGES_DIR} not found.${NC}"
    exit 1
fi

if [ ! -f "${SCANNER_BINARY}" ]; then
    echo -e "${RED}ERROR: ${SCANNER_BINARY} not found in current directory.${NC}"
    exit 1
fi
chmod +x "${SCANNER_BINARY}"

# Setup Global Config
[ -f "${SCANNER_CONFIG}" ] && export SECURITY_SCANNER_CONFIG_FILE="${SCANNER_CONFIG}"

# Initialize Report
mkdir -p "${REPORT_DIR}"
cat > "${CONSOLIDATED_REPORT}" << EOF
================================================================================
CONSOLIDATED SECURITY SCAN REPORT (BINARY & MULTI-ARCH)
Generated: $(date)
================================================================================
EOF

# --- Scan Loop ---
total=0; binary_scans=0; container_scans=0; with_vulns=0
ABS_IMAGES_DIR="$(cd "${IMAGES_DIR}" && pwd)"

for file in "${IMAGES_DIR}"/*; do
    [ -f "${file}" ] || continue
    filename=$(basename "${file}")
    ext="${filename##*.}"
    total=$((total + 1))
    
    echo -e "\n${BLUE}[$total] Target: ${filename}${NC}"
    echo -e "################################################################################" >> "${CONSOLIDATED_REPORT}"
    echo -e "# TARGET: ${filename}" >> "${CONSOLIDATED_REPORT}"

    # ---------------------------------------------------------
    # CASE 1: CONTAINER SCAN (.tar files)
    # ---------------------------------------------------------
    if [[ "${filename}" == *".docker"* ]] || [[ "${ext}" == "tar" ]]; then
        echo -e "  -> Mode: ${YELLOW}CONTAINER SCAN${NC}"
        SCAN_EXIT_CODE=0

        # Special logic for UBI images (to prevent Alpine misdetection)
        if [[ "${filename}" == *"ubi"* ]]; then
            echo "     💾 Loading UBI image into Docker for OS-specific detection..."
            LOAD_OUTPUT=$(docker load -i "${file}")
            IMAGE_TAG=$(echo "$LOAD_OUTPUT" | grep "Loaded image" | awk '{print $NF}')
            
            echo "MODE: CONTAINER SCAN (UBI/DAEMON)" >> "${CONSOLIDATED_REPORT}"
            ${SCANNER_BINARY} container "docker://${IMAGE_TAG}" >> "${CONSOLIDATED_REPORT}" 2>&1 || SCAN_EXIT_CODE=$?
            
            # Cleanup daemon image
            docker rmi "${IMAGE_TAG}" > /dev/null 2>&1 || true
        else
            # Standard/Alpine Scan
            echo "MODE: CONTAINER SCAN (TARBALL)" >> "${CONSOLIDATED_REPORT}"
            ${SCANNER_BINARY} container "${file}" >> "${CONSOLIDATED_REPORT}" 2>&1 || SCAN_EXIT_CODE=$?
        fi
        
        container_scans=$((container_scans + 1))
        [ ${SCAN_EXIT_CODE} -ne 0 ] && with_vulns=$((with_vulns + 1))

    # ---------------------------------------------------------
    # CASE 2: BINARY SCAN (.deb or .rpm files)
    # ---------------------------------------------------------
    elif [[ "${ext}" == "deb" ]] || [[ "${ext}" == "rpm" ]]; then
        echo -e "  -> Mode: ${GREEN}BINARY SCAN${NC}"
        echo "MODE: BINARY SCAN (EXTRACTED PACKAGE)" >> "${CONSOLIDATED_REPORT}"
        
        extract_dir="${TEMP_DIR}/${filename}_extract"
        mkdir -p "${extract_dir}"
        
        extracted=false
        if [ "${ext}" = "deb" ]; then
            if command -v dpkg-deb &>/dev/null; then
                dpkg-deb -x "${file}" "${extract_dir}" 2>/dev/null && extracted=true
            else
                # Fallback for systems without dpkg-deb (e.g. macOS)
                (cd "${extract_dir}" && ar x "${ABS_IMAGES_DIR}/${filename}" 2>/dev/null && \
                (tar -xf data.tar.* 2>/dev/null || tar -xf data.tar 2>/dev/null)) && extracted=true
            fi
        else
            # RPM Extraction
            if command -v rpm2cpio &>/dev/null; then
                (cd "${extract_dir}" && rpm2cpio "${ABS_IMAGES_DIR}/${filename}" | cpio -idm 2>/dev/null) && extracted=true
            fi
        fi

        if [ "$extracted" = true ]; then
            binary_path=$(find "${extract_dir}" -name "consul" -type f | head -1)
            if [ -n "$binary_path" ]; then
                b_exit=0
                # Scan binary & sanitize temp paths in report
                ${SCANNER_BINARY} binary "${binary_path}" 2>&1 | \
                    sed "s|${TEMP_DIR}/[^/ ]*/|${filename}/|g" >> "${CONSOLIDATED_REPORT}" || b_exit=$?
                
                binary_scans=$((binary_scans + 1))
                [ ${b_exit} -ne 0 ] && with_vulns=$((with_vulns + 1))
            else
                echo -e "${RED}     ✗ Binary 'consul' not found in package.${NC}"
            fi
        else
            echo -e "${RED}     ✗ Extraction Failed.${NC}"
        fi
    fi
    echo -e "================================================================================" >> "${CONSOLIDATED_REPORT}"
done

# --- Final Summary ---
echo -e "\n${BLUE}========================================${NC}"
echo -e "  SCAN SUMMARY"
echo -e "========================================${NC}"
echo "Total Artifacts:    ${total}"
echo "Container Scans:    ${container_scans}"
echo "Binary Scans:       ${binary_scans}"
echo "Vulnerable Targets: ${with_vulns}"
echo -e "\nReport: ${CONSOLIDATED_REPORT}"

[ ${with_vulns} -eq 0 ] && exit 0 || exit 1
