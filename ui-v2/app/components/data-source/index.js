import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';

import Ember from 'ember';
/**
 * Utility function to set, but actually replace if we should replace
 * then call a function on the thing to be replaced (usually a clean up function)
 *
 * @param obj - target object with the property to replace
 * @param prop {string} - property to replace on the target object
 * @param value - value to use for replacement
 * @param destroy {(prev: any, value: any) => any} - teardown function
 */
const replace = function(
  obj,
  prop,
  value,
  destroy = (prev = null, value) => (typeof prev === 'function' ? prev() : null)
) {
  const prev = obj[prop];
  if (prev !== value) {
    destroy(prev, value);
  }
  return set(obj, prop, value);
};

/**
 * @module DataSource
 *
 * The DataSource component manages opening and closing data sources via an injectable data service.
 * Data sources are only opened only if the component is visible in the viewport (using IntersectionObserver).
 *
 * Sources returned by the data service should follow an EventTarget/EventSource API.
 * Management of the caching/usage/counting etc of sources should be done in the data service,
 * not the component.
 *
 * @example ```javascript
 *   <DataSource
 *      src="/dc-1/~nspace/services"
 *      onchange={{action (mut items) value='data'}}
 *      onerror={{action (mut error) value='error'}}
 *   />```
 *
 * @param src {string} - An identifier used to determine the source of the data. This is passed
 * @param loading {string} - Either `eager` or `lazy`, lazy will only load the data once the component
 *                           is in the viewport
 * @param onchange {function=} - An action called when the data changes.
 * @param onerror {function=} - An action called on error
 *
 */
export default Component.extend({
  tagName: '',

  data: service('data-source/service'),
  dom: service('dom'),
  logger: service('logger'),

  onchange: function(e) {},
  onerror: function(e) {},

  loading: 'eager',

  isIntersecting: false,

  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
    this._lazyListeners = this.dom.listeners();
  },
  willDestroy: function() {
    this.actions.close.apply(this);
    this._listeners.remove();
    this._lazyListeners.remove();
  },

  didInsertElement: function() {
    this._super(...arguments);
    if (this.loading === 'lazy') {
      this._lazyListeners.add(
        this.dom.isInViewport(this.element, inViewport => {
          set(this, 'isIntersecting', inViewport || Ember.testing);
          if (!this.isIntersecting) {
            this.actions.close.bind(this)();
          } else {
            this.actions.open.bind(this)();
          }
        })
      );
    }
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    if (this.loading === 'eager') {
      this._lazyListeners.remove();
    }
    if (this.loading === 'eager' || this.isIntersecting) {
      this.actions.open.bind(this)();
    }
  },
  actions: {
    // keep this argumentless
    open: function() {
      // get a new source and replace the old one, cleaning up as we go
      const source = replace(this, 'source', this.data.open(this.src, this), (prev, source) => {
        // Makes sure any previous source (if different) is ALWAYS closed
        this.data.close(prev, this);
      });
      const error = err => {
        try {
          this.onerror(err);
          this.logger.execute(err);
        } catch (err) {
          this.logger.execute(err);
        }
      };
      // set up the listeners (which auto cleanup on component destruction)
      const remove = this._listeners.add(this.source, {
        message: e => {
          try {
            this.onchange(e);
          } catch (err) {
            error(err);
          }
        },
        error: e => error(e),
      });
      replace(this, '_remove', remove);
      // dispatch the current data of the source if we have any
      if (typeof source.getCurrentEvent === 'function') {
        const currentEvent = source.getCurrentEvent();
        if (currentEvent) {
          try {
            this.onchange(currentEvent);
          } catch (err) {
            error(err);
          }
        }
      }
    },
    // keep this argumentless
    close: function() {
      if (typeof this.source !== 'undefined') {
        this.data.close(this.source, this);
        replace(this, '_remove', undefined);
        set(this, 'source', undefined);
      }
    },
  },
});
