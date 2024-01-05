/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (collection, text) =>
  (scope = '.consul-health-check-list') => {
    return collection({
      scope,
      itemScope: 'li',
      item: {
        name: text('header h2'),
        type: text('[data-health-check-type]'),
        exposed: text('[data-test-exposed]'),
      },
    });
  };
