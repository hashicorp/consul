/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';
import { guidFor } from '@ember/object/internals';

// selecting
import qsaFactory from 'consul-ui/utils/dom/qsa-factory';
// TODO: sibling and closest seem to have 'PHP-like' guess the order arguments
// ie. one `string, element` and the other has `element, string`
// see if its possible to standardize
import sibling from 'consul-ui/utils/dom/sibling';
import closest from 'consul-ui/utils/dom/closest';
import isOutside from 'consul-ui/utils/dom/is-outside';
import getComponentFactory from 'consul-ui/utils/dom/get-component-factory';

// events
import normalizeEvent from 'consul-ui/utils/dom/normalize-event';
import createListeners from 'consul-ui/utils/dom/create-listeners';
import clickFirstAnchorFactory from 'consul-ui/utils/dom/click-first-anchor';

// ember-eslint doesn't like you using a single $ so use double
// use $_ for components
const $$ = qsaFactory();
let $_;
let inViewportCallbacks;
const clickFirstAnchor = clickFirstAnchorFactory(closest);
export default class DomService extends Service {
  @service('-document') doc;

  constructor(owner) {
    super(...arguments);
    inViewportCallbacks = new WeakMap();
    $_ = getComponentFactory(owner);
  }

  willDestroy() {
    super.willDestroy(...arguments);
    inViewportCallbacks = null;
    $_ = null;
  }

  document() {
    return this.doc;
  }

  viewport() {
    return this.doc.defaultView;
  }

  guid(el) {
    return guidFor(el);
  }

  focus($el) {
    if (typeof $el === 'string') {
      $el = this.element($el);
    }
    if (typeof $el !== 'undefined') {
      let previousIndex = $el.getAttribute('tabindex');
      $el.setAttribute('tabindex', '0');
      $el.focus();
      if (previousIndex === null) {
        $el.removeAttribute('tabindex');
      } else {
        $el.setAttribute('tabindex', previousIndex);
      }
    }
  }

  // TODO: should this be here? Needs a better name at least
  clickFirstAnchor = clickFirstAnchor;

  closest = closest;
  sibling = sibling;
  isOutside = isOutside;
  normalizeEvent = normalizeEvent;

  setEventTargetProperty(e, property, cb) {
    const target = e.target;
    return new Proxy(e, {
      get: function (obj, prop, receiver) {
        if (prop === 'target') {
          return new Proxy(target, {
            get: function (obj, prop, receiver) {
              if (prop === property) {
                return cb(e.target[property]);
              }
              return target[prop];
            },
          });
        }
        return Reflect.get(...arguments);
      },
    });
  }

  setEventTargetProperties(e, propObj) {
    const target = e.target;
    return new Proxy(e, {
      get: function (obj, prop, receiver) {
        if (prop === 'target') {
          return new Proxy(target, {
            get: function (obj, prop, receiver) {
              if (typeof propObj[prop] !== 'undefined') {
                return propObj[prop](e.target);
              }
              return target[prop];
            },
          });
        }
        return Reflect.get(...arguments);
      },
    });
  }

  listeners = createListeners;

  root() {
    return this.doc.documentElement;
  }

  // TODO: Should I change these to use the standard names
  // even though they don't have a standard signature (querySelector*)
  elementById(id) {
    return this.doc.getElementById(id);
  }

  elementsByTagName(name, context) {
    context = typeof context === 'undefined' ? this.doc : context;
    return context.getElementsByTagName(name);
  }

  elements(selector, context) {
    // don't ever be tempted to [...$$()] here
    // it should return a NodeList
    return $$(selector, context);
  }

  element(selector, context) {
    if (selector.substr(0, 1) === '#') {
      return this.elementById(selector.substr(1));
    }
    // TODO: This can just use querySelector
    return [...$$(selector, context)][0];
  }

  // ember components aren't strictly 'dom-like'
  // but if you think of them as a web component 'shim'
  // then it makes more sense to think of them as part of the dom
  // with traditional/standard web components you wouldn't actually need this
  // method as you could just get to their methods from the dom element
  component(selector, context) {
    if (typeof selector !== 'string') {
      return $_(selector);
    }
    return $_(this.element(selector, context));
  }

  components(selector, context) {
    return [...this.elements(selector, context)]
      .map(function (item) {
        return $_(item);
      })
      .filter(function (item) {
        return item != null;
      });
  }

  isInViewport($el, cb, threshold = 0) {
    inViewportCallbacks.set($el, cb);
    let observer = new IntersectionObserver(
      (entries, observer) => {
        entries.map((item) => {
          const cb = inViewportCallbacks.get(item.target);
          if (typeof cb === 'function') {
            cb(item.isIntersecting);
          }
        });
      },
      {
        rootMargin: '0px',
        threshold: threshold,
      }
    );
    observer.observe($el); // eslint-disable-line ember/no-observers
    // observer.unobserve($el);
    return () => {
      observer.unobserve($el); // eslint-disable-line ember/no-observers
      if (inViewportCallbacks) {
        inViewportCallbacks.delete($el);
      }
      observer.disconnect(); // eslint-disable-line ember/no-observers
      observer = null;
    };
  }
}
