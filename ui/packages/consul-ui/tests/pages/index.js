/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, collection) {
  return {
    visit: visitable('/'),
    dcs: collection('[data-test-datacenter-list]'),
  };
}
