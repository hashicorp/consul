/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (visitable, collection) {
  return {
    visit: visitable('/'),
    dcs: collection('[data-test-datacenter-list]'),
  };
}
