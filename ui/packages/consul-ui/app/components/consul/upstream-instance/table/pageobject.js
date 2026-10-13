/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, text) =>
  (scope = '.consul-upstream-instance-table') => {
    return {
      scope,
      item: collection('tbody tr', {
        name: text('[data-test-upstream-name]'),
        nspace: text('[data-test-upstream-namespace]'),
        partition: text('[data-test-upstream-partition]'),
        datacenter: text('[data-test-upstream-datacenter]'),
        address: text('[data-test-upstream-address]'),
        mode: text('[data-test-upstream-mode]'),
      }),
    };
  };
