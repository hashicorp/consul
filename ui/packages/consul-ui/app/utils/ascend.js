/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (path, num) {
  const parts = path.split('/');
  return parts.length > num ? parts.slice(0, -num).concat('').join('/') : '';
}
