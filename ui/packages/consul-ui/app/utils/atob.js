/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import base64js from 'base64-js';
export default function (str, encoding = 'utf-8') {
  // decode
  const bytes = base64js.toByteArray(str);
  return new TextDecoder(encoding).decode(bytes);
}
