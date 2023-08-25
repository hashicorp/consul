/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Boolean if the key is a "folder" or not, i.e is a nested key
// that feels like a folder.
export default function (path = '') {
  return path.slice(-1) === '/';
}
