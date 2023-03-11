/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { get, set } from '@ember/object';

const proxies = {};
export default function (ObjProxy, ArrProxy, createListeners) {
  return function (source, data = []) {
    let Proxy = ObjProxy;
    // TODO: When is data ever a string?
    let type = 'object';
    if (typeof data !== 'string' && typeof get(data, 'length') !== 'undefined') {
      Proxy = ArrProxy;
      type = 'array';
      data = data.filter(function (item) {
        return !get(item, 'isDestroyed') && !get(item, 'isDeleted') && get(item, 'isLoaded');
      });
    }
    if (typeof proxies[type] === 'undefined') {
      proxies[type] = Proxy.extend({
        init: function () {
          this.listeners = createListeners();
          this.listeners.add(this._source, 'message', (e) => set(this, 'content', e.data));
          this._super(...arguments);
        },
        addEventListener: function (type, handler) {
          this.listeners.add(this._source, type, handler);
        },
        getCurrentEvent: function () {
          return this._source.getCurrentEvent(...arguments);
        },
        removeEventListener: function () {
          return this._source.removeEventListener(...arguments);
        },
        dispatchEvent: function () {
          return this._source.dispatchEvent(...arguments);
        },
        close: function () {
          return this._source.close(...arguments);
        },
        open: function () {
          return this._source.open(...arguments);
        },
        willDestroy: function () {
          this._super(...arguments);
          this.close();
          this.listeners.remove();
        },
      });
    }
    return proxies[type].create({
      content: data,
      _source: source,
      configuration: source.configuration,
    });
  };
}
