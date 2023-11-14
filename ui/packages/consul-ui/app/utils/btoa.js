/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import base64js from 'base64-js';
export default function (str) {
  // encode
  const bytes = new TextEncoder().encode(str);
  return base64js.fromByteArray(bytes);
}
