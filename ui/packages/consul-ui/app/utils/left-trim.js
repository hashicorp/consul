/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function leftTrim(str = '', search = '') {
  return str.indexOf(search) === 0 ? str.substr(search.length) : str;
}
