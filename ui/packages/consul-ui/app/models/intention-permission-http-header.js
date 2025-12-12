/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import { tracked } from '@glimmer/tracking';

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

  @tracked _headerTypeManual;

  get Value() {
    return this.Exact || this.Prefix || this.Suffix || this.Contains || this.Regex || this.Present;
  }
  @attr('boolean') IgnoreCase;

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
