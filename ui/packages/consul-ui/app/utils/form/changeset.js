/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { get } from '@ember/object';
import { EmberChangeset as Changeset } from 'ember-changeset';
const CHANGES = '_changes';
export default class extends Changeset {
  pushObject(prop, value) {
    let val;
    if (typeof get(this, `${CHANGES}.${prop}`) === 'undefined') {
      val = get(this, `data.${prop}`);
      if (!val) {
        val = [];
      } else {
        val = val.toArray();
      }
    } else {
      val = this.get(prop).slice(0);
    }
    val.push(value);
    this.set(`${prop}`, val);
  }
  removeObject(prop, value) {
    let val;
    if (typeof get(this, `${CHANGES}.${prop}`) === 'undefined') {
      val = get(this, `data.${prop}`);
      if (typeof val === 'undefined') {
        val = [];
      } else {
        val = val.toArray();
      }
    } else {
      val = this.get(prop).slice(0);
    }
    const pos = val.indexOf(value);
    if (pos !== -1) {
      val.splice(pos, 1);
    }
    this.set(`${prop}`, val);
  }
}
