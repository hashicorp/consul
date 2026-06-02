/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function leftTrim(str = '', search = '') {
  return str.indexOf(search) === 0 ? str.substr(search.length) : str;
}
