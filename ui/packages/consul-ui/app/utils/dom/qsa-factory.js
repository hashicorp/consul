/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (doc = document) {
  return function (sel, context = doc) {
    return context.querySelectorAll(sel);
  };
}
