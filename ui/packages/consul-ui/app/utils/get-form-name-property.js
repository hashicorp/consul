/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (name) {
  if (name.indexOf('[') !== -1) {
    return name.match(/(.*)\[(.*)\]/).slice(1);
  }
  return ['', name];
}
