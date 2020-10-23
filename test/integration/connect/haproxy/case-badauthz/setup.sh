#!/bin/bash

set -eEuo pipefail

wait_for_catalog_service_register s1 primary
wait_for_catalog_service_register s1-sidecar-proxy primary
wait_for_catalog_service_register s2 primary
wait_for_catalog_service_register s2-sidecar-proxy primary
