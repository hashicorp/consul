/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, text) =>
  (scope = '.consul-exposed-path-table') => {
    return {
      scope,
      item: collection('tbody tr', {
        combinedAddress: text('[data-test-combined-address]'),
        protocol: text('[data-test-exposed-path-protocol]'),
        listenerPort: text('[data-test-exposed-path-listener-port]'),
        localPath: text('[data-test-exposed-path-local-path]'),
        path: text('[data-test-exposed-path-path]'),
      }),
    };
  };
