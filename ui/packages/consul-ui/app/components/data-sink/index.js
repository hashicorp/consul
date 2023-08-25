/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set, get, computed } from '@ember/object';

import { once } from 'consul-ui/utils/dom/event-source';

export default Component.extend({
  tagName: '',

  service: service('data-sink/service'),
  dom: service('dom'),
  logger: service('logger'),

  onchange: function (e) {},
  onerror: function (e) {},

  state: computed('instance', 'instance.{dirtyType,isSaving}', function () {
    let id;
    const isSaving = get(this, 'instance.isSaving');
    const dirtyType = get(this, 'instance.dirtyType');
    if (typeof isSaving === 'undefined' && typeof dirtyType === 'undefined') {
      id = 'idle';
    } else {
      switch (dirtyType) {
        case 'created':
          id = isSaving ? 'creating' : 'create';
          break;
        case 'updated':
          id = isSaving ? 'updating' : 'update';
          break;
        case 'deleted':
        case undefined:
          id = isSaving ? 'removing' : 'remove';
          break;
      }
      id = `active.${id}`;
    }
    return {
      matches: (name) => id.indexOf(name) !== -1,
    };
  }),

  init: function () {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
  },
  willDestroyElement: function () {
    this._super(...arguments);
    this._listeners.remove();
  },
  source: function (cb) {
    const source = once(cb);
    const error = (err) => {
      set(this, 'instance', undefined);
      try {
        this.onerror(err);
        this.logger.execute(err);
      } catch (err) {
        this.logger.execute(err);
      }
    };
    this._listeners.add(source, {
      message: (e) => {
        try {
          set(this, 'instance', undefined);
          this.onchange(e);
        } catch (err) {
          error(err);
        }
      },
      error: (e) => error(e),
    });
    return source;
  },
  didInsertElement: function () {
    this._super(...arguments);
    if (typeof this.data !== 'undefined' || typeof this.item !== 'undefined') {
      this.actions.open.apply(this, [this.data, this.item]);
    }
  },
  persist: function (data, instance) {
    if (typeof data !== 'undefined') {
      set(this, 'instance', this.service.prepare(this.sink, data, instance));
    } else {
      set(this, 'instance', instance);
    }
    this.source(() => this.service.persist(this.sink, this.instance));
  },
  remove: function (instance) {
    set(this, 'instance', instance);
    this.source(() => this.service.remove(this.sink, instance));
  },
  actions: {
    open: function (data, item) {
      if (item instanceof Event) {
        item = undefined;
      }
      if (typeof data === 'undefined' && typeof item === 'undefined') {
        throw new Error('You must specify data to save, or null to remove');
      }
      // potentially allow {} and "" as 'remove' flags
      if (data === null || data === '') {
        this.remove(item);
      } else {
        this.persist(data, item);
      }
    },
  },
});
