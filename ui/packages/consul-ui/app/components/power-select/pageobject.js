/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { clickable, isPresent } from 'ember-cli-page-object';

export default (options) => {
  return {
    present: isPresent('.ember-power-select-trigger'),
    click: clickable('.ember-power-select-trigger'),
    option: {
      resetScope: true,
      ...options.reduce((prev, item, i) => {
        prev[item] = {
          click: clickable(`[data-option-index='${i}']`),
        };
        return prev;
      }, {}),
    },
  };
};
