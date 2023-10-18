/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { runInDebug } from '@ember/debug';
import MultiMap from 'mnemonist/multi-map';

// keep a record or attrs
const attrs = new Map();

// keep a record of hashes privately
const wm = new WeakMap();

export default class DocumentAttrsHelper extends Helper {
  @service('-document') document;

  compute(params, hash) {
    this.synchronize(this.document.documentElement, hash);
  }

  willDestroy() {
    this.synchronize(this.document.documentElement);
    wm.delete(this);
  }

  synchronize(root, hash) {
    const prev = wm.get(this);
    if (prev) {
      // if this helper was already setting a property then remove them from
      // our book keeping
      Object.entries(prev).forEach(([key, value]) => {
        let map = attrs.get(key);

        if (typeof map !== 'undefined') {
          [...new Set(value.split(' '))].map((val) => map.remove(val, this));
        }
      });
    }
    if (hash) {
      // if we are setting more properties add them to our book keeping
      wm.set(this, hash);
      [...Object.entries(hash)].forEach(([key, value]) => {
        let values = attrs.get(key);
        if (typeof values === 'undefined') {
          values = new MultiMap(Set);
          attrs.set(key, values);
        }
        [...new Set(value.split(' '))].map((val) => {
          if (values.count(val) === 0) {
            values.set(val, null);
          }
          values.set(val, this);
        });
      });
    }
    [...attrs.entries()].forEach(([attr, values]) => {
      let type = 'attr';
      if (attr === 'class') {
        type = attr;
      } else if (attr.startsWith('data-')) {
        type = 'data';
      }
      // go through our list of properties and synchronize the DOM
      // properties with our properties
      [...values.keys()].forEach((value) => {
        if (values.count(value) === 1) {
          switch (type) {
            case 'class':
              root.classList.remove(value);
              break;
            case 'data':
            default:
              runInDebug(() => {
                throw new Error(`${type} is not implemented yet`);
              });
          }
          values.delete(value);
          // remove the property if it has no values
          if (values.size === 0) {
            attrs.delete(attr);
          }
        } else {
          switch (type) {
            case 'class':
              root.classList.add(value);
              break;
            case 'data':
            default:
              runInDebug(() => {
                throw new Error(`${type} is not implemented yet`);
              });
          }
        }
      });
    });
    return attrs;
  }
}
