/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// The name a key is listed under, i.e. its last non-empty path segment:
// 'a/b/c' -> 'c' and the folder 'a/b/' -> 'b'. Undefined at the root.
export default function (path = '') {
  const parts = path.split('/').filter(Boolean);
  return parts[parts.length - 1];
}
