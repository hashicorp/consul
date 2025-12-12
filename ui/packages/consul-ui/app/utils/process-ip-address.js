/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export function processIpAddress(ip) {
  // Simple IPv4 validation
  const ipv4Pattern =
    /^(25[0-5]|2[0-4]\d|1\d\d|\d\d|\d)\.(25[0-5]|2[0-4]\d|1\d\d|\d\d|\d)\.(25[0-5]|2[0-4]\d|1\d\d|\d\d|\d)\.(25[0-5]|2[0-4]\d|1\d\d|\d\d|\d)$/;

  // Basic IPv6 pattern (loose, just enough to pass to URL)
  const ipv6Pattern = /^[0-9a-fA-F:]+$/;

  // FQDN validation (RFC 1035, basic)
  const fqdnPattern =
    /^(?=.{1,253}$)(?!-)[A-Za-z0-9-]{1,63}(?<!-)(\.(?!-)[A-Za-z0-9-]{1,63}(?<!-))*\.?[A-Za-z]{2,}$/;

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

  if (fqdnPattern.test(ip)) {
    return ip; // Valid FQDN, return as-is
  }

  return null; // Not valid IPv4, IPv6, or FQDN
}
