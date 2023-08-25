/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
