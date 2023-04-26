/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (str) {
  return `${str.substr(0, 1).toUpperCase()}${str.substr(1)}`;
}
