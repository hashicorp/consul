/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default (collection, clickable, attribute, isPresent, deletable) =>
  (scope = '.consul-intention-list') => {
    const row = {
      source: attribute('data-test-intention-source', '[data-test-intention-source]'),
      destination: attribute(
        'data-test-intention-destination',
        '[data-test-intention-destination]'
      ),
      action: attribute('data-test-intention-action', '[data-test-intention-action]'),
      intention: clickable('a'),
      actions: clickable('label'),
      ...deletable(),
    };
    return {
      scope: scope,
      customResourceNotice: isPresent('.consul-intention-notice-custom-resource'),
      intentions: collection('[data-test-tabular-row]', row),
    };
  };
