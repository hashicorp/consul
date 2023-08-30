/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (name) {
  if (name.indexOf('[') !== -1) {
    return name.match(/(.*)\[(.*)\]/).slice(1);
  }
  return ['', name];
}
