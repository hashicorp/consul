import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';

import WithListeners from 'consul-ui/mixins/with-listeners';

// Utility function to set, but actually replace if we should replace
// then call a function on the thing to be replaced (usually a clean up function)
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
 * @example ```js
 *   {{data-source
 *      src="/dc-1/services/*"
 *      onchange={{action (mut items) value='data'}}
 *      onerror={{action (mut error) value='error'}}
 *   /}}```
 *
 * @param src {String} - An identitier used to determine the source of the data. This is passed
 *                       to the data service for it to determine how to fetch the data.
 * @param onchange=null {Func} - An action called when the data changes.
 * @param onerror=null {Func} - An action called on error
 *
 */
export default Component.extend(WithListeners, {
  tagName: 'span',

  // TODO: can be injected with a simpler non-blocking
  // data service if we turn off blocking queries completely at runtime
  data: service('blocking'),

  onchange: function() {},
  onerror: function() {},

  didInsertElement: function() {
    this._super(...arguments);
    const options = {
      rootMargin: '0px',
      threshold: 1.0,
    };

    const observer = new IntersectionObserver((entries, observer) => {
      entries.map(item => {
        set(this, 'isIntersecting', item.isIntersecting);
        if (!item.isIntersecting) {
          this.actions.close.bind(this)();
        } else {
          this.actions.open.bind(this)();
        }
      });
    }, options);
    observer.observe(this.element); // eslint-disable-line ember/no-observers
    this.listen(() => {
      this.actions.close.bind(this)();
      observer.disconnect(); // eslint-disable-line ember/no-observers
    });
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    if (this.element && this.isIntersecting) {
      this.actions.open.bind(this)();
    }
  },
  actions: {
    // keep this argumentless
    open: function() {
      const src = this.src;
      const filter = this.filter;

      // get a new source and replace the old one, cleaning up as we go
      const source = replace(
        this,
        'source',
        this.data.open(`${src}${filter ? `?filter=${filter}` : ``}`, this),
        (prev, source) => {
          // Makes sure any previous source (if different) is ALWAYS closed
          this.data.close(prev, this);
        }
      );
      // set up the listeners (which auto cleanup on component destruction)
      const remove = this.listen(source, {
        message: e => this.onchange(e),
        error: e => this.onerror(e),
      });
      replace(this, '_remove', remove);
      // dispatch the current data of the source if we have any
      if (typeof source.getCurrentEvent === 'function') {
        const currentEvent = source.getCurrentEvent();
        if (currentEvent) {
          this.onchange(currentEvent);
        }
      }
    },
    // keep this argumentless
    close: function() {
      this.data.close(this.source, this);
      replace(this, '_remove', null);
      set(this, 'source', undefined);
    },
  },
});
