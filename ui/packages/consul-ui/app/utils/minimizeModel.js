/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { get } from '@ember/object';

export default function (arr) {
  if (Array.isArray(arr)) {
    return arr
      .filter(function (item) {
        // Just incase, don't save any models that aren't saved
        return !get(item, 'isNew');
      })
      .map(function (item) {
        return {
          ID: get(item, 'ID'),
          Name: get(item, 'Name'),
        };
      });
  }
}
