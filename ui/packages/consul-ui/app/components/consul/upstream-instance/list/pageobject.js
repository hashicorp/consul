/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, text) =>
  (scope = '.consul-upstream-instance-list') => {
    return {
      scope,
      item: collection('li', {
        name: text('.header p'),
        nspace: text('.nspace dd'),
        datacenter: text('.datacenter dd'),
        localAddress: text('.local-address dd'),
      }),
    };
  };
