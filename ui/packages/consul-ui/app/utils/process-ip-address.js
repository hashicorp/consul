/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export function processIpAddress(ip) {
  // Simple IPv4 validation
  const ipv4Pattern =
    /^(25[0-5]|2[0-4]\d|1\d\d|\d\d|\d)\.(25[0-5]|2[0-4]\d|1\d\d|\d\d|\d)\.(25[0-5]|2[0-4]\d|1\d\d|\d\d|\d)\.(25[0-5]|2[0-4]\d|1\d\d|\d\d|\d)$/;

  // Basic IPv6 pattern (loose, just enough to pass to URL)
  const ipv6Pattern = /^[0-9a-fA-F:]+$/;

  if (ipv4Pattern.test(ip)) {
    return ip; // Valid IPv4, return as-is
  }

  if (ipv6Pattern.test(ip)) {
    try {
      const url = new URL(`http://[${ip}]/`);
      return url.hostname; // Returns collapsed IPv6
    } catch (e) {
      return null; // Invalid IPv6
    }
  }

  return null; // Not valid IPv4 or IPv6
}
