/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (arr = []) {
  return arr.map((item) => `template-${item}`);
}
