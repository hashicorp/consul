/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

export const schema = {
  Name: {
    required: true,
  },
  HeaderType: {
    allowedValues: ['Exact', 'Prefix', 'Suffix', 'Contains', 'Regex', 'Present'],
  },
};

export default class IntentionPermission extends Fragment {
  @attr('string') Name;

  @attr('string') Exact;
  @attr('string') Prefix;
  @attr('string') Suffix;
  @attr('string') Contains;
  @attr('string') Regex;
  // this is a boolean but we don't want it to automatically be set to false
  @attr() Present;

  @or(...schema.HeaderType.allowedValues) Value;
  @attr('boolean') IgnoreCase;

  @computed(...schema.HeaderType.allowedValues)
  get HeaderType() {
    // Use manual override if one was set
    if (this._headerTypeManual !== undefined) {
      return this._headerTypeManual;
    }
    // Original logic: find first defined variant field
    return schema.HeaderType.allowedValues.find((prop) => typeof this[prop] !== 'undefined');
  }

  set HeaderType(value) {
    // Store manual override
    this._headerTypeManual = value;
  }
}
