/* eslint no-console: ["error", { allow: ["debug"] }] */
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { action, get } from '@ember/object';
import { schedule } from '@ember/runloop';
import { runInDebug } from '@ember/debug';

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
  return (obj[prop] = value);
};

const noop = () => {};
const optional = op => (typeof op === 'function' ? op : noop);

// possible values for @loading=""
const LOADING = ['eager', 'lazy'];

export default class DataSource extends Component {
  @service('data-source/service') dataSource;
  @service('dom') dom;
  @service('logger') logger;

  @tracked isIntersecting = false;
  @tracked data;
  @tracked error;

  constructor(owner, args) {
    super(...arguments);
    this._listeners = this.dom.listeners();
    this._lazyListeners = this.dom.listeners();
  }

  get loading() {
    return LOADING.includes(this.args.loading) ? this.args.loading : LOADING[0];
  }

  get disabled() {
    return typeof this.args.disabled !== 'undefined' ? this.args.disabled : false;
  }

  onchange(e) {
    this.error = undefined;
    this.data = e.data;
    optional(this.args.onchange)(e);
  }

  onerror(e) {
    this.error = e.error || e;
    optional(this.args.onerror)(e);
  }

  @action
  connect($el) {
    // $el is only a DOM node when loading = lazy
    // otherwise its an array from the did-insert-helper
    if (!Array.isArray($el)) {
      this._lazyListeners.add(
        this.dom.isInViewport($el, inViewport => {
          this.isIntersecting = inViewport;
          if (!this.isIntersecting) {
            this.close();
          } else {
            this.open();
          }
        })
      );
    } else {
      this._lazyListeners.remove();
      this.open();
    }
  }

  @action
  disconnect() {
    // TODO: Should we be doing this here? Fairly sure we should be so if this
    // TODO gets old enough (6 months/ 1 year or so) feel free to remove
    if (
      typeof this.data !== 'undefined' &&
      typeof this.data.length === 'undefined' &&
      typeof this.data.rollbackAttributes === 'function'
    ) {
      this.data.rollbackAttributes();
    }
    this.close();
    this._listeners.remove();
    this._lazyListeners.remove();
  }

  @action
  attributeChanged([name, value]) {
    switch (name) {
      case 'src':
        if (this.loading === 'eager' || this.isIntersecting) {
          this.open();
        }
        break;
    }
  }

  // keep this argumentless
  @action
  open() {
    const src = this.args.src;
    // get a new source and replace the old one, cleaning up as we go
    const source = replace(
      this,
      'source',
      this.dataSource.open(src, this, this.open),
      (prev, source) => {
        // Makes sure any previous source (if different) is ALWAYS closed
        this.dataSource.close(prev, this);
      }
    );
    const error = err => {
      try {
        const error = get(err, 'error.errors.firstObject') || {};
        if (get(error, 'status') !== '429') {
          this.onerror(err);
        }
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
      error: e => {
        error(e);
      },
    });
    replace(this, '_remove', remove);
    // dispatch the current data of the source if we have any
    if (typeof source.getCurrentEvent === 'function') {
      const currentEvent = source.getCurrentEvent();
      if (currentEvent) {
        let method;
        if (typeof currentEvent.error !== 'undefined') {
          method = 'onerror';
          this.error = currentEvent.error;
        } else {
          this.error = undefined;
          this.data = currentEvent.data;
          method = 'onchange';
        }

        // avoid the re-render error
        schedule('afterRender', () => {
          try {
            this[method](currentEvent);
          } catch (err) {
            error(err);
          }
        });
      }
    }
  }

  @action
  async invalidate() {
    this.source.readyState = 2;
    this.disconnect();
    schedule('afterRender', () => {
      // TODO: Support lazy data-sources by keeping a reference to $el
      runInDebug(_ =>
        console.debug(
          `Invalidation is only supported for non-lazy data sources. If you want to use this you should fixup support for lazy data sources`
        )
      );
      this.connect([]);
    });
  }

  // keep this argumentless
  @action
  close() {
    if (typeof this.source !== 'undefined') {
      this.dataSource.close(this.source, this);
      replace(this, '_remove', undefined);
      this.source = undefined;
    }
  }
}
