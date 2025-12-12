/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (path, num) {
  const parts = path.split('/');
  return parts.length > num ? parts.slice(0, -num).concat('').join('/') : '';
}
