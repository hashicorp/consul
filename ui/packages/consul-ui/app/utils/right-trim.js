/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function rightTrim(str = '', search = '') {
  const pos = str.length - search.length;
  if (pos >= 0) {
    return str.lastIndexOf(search) === pos ? str.substr(0, pos) : str;
  }
  return str;
}
