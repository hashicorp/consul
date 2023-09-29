/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (arr = []) {
  return arr.map((item) => `template-${item}`);
}
