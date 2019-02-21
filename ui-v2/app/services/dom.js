import Service from '@ember/service';
import { getOwner } from '@ember/application';
import { get } from '@ember/object';

// selecting
import qsaFactory from 'consul-ui/utils/dom/qsa-factory';
// TODO: sibling and closest seem to have 'PHP-like' guess the order arguments
// ie. one `string, element` and the other has `element, string`
// see if its possible to standardize
import sibling from 'consul-ui/utils/dom/sibling';
import closest from 'consul-ui/utils/dom/closest';
import getComponentFactory from 'consul-ui/utils/dom/get-component-factory';

// events
import normalizeEvent from 'consul-ui/utils/dom/normalize-event';
import createListeners from 'consul-ui/utils/dom/create-listeners';
import clickFirstAnchorFactory from 'consul-ui/utils/dom/click-first-anchor';

// ember-eslint doesn't like you using a single $ so use double
// use $_ for components
const $$ = qsaFactory();
let $_;
const clickFirstAnchor = clickFirstAnchorFactory(closest);
export default Service.extend({
  doc: document,
  win: window,
  init: function() {
    this._super(...arguments);
    $_ = getComponentFactory(getOwner(this));
  },
  document: function() {
    return get(this, 'doc');
  },
  viewport: function() {
    return get(this, 'win');
  },
  // TODO: should this be here? Needs a better name at least
  clickFirstAnchor: clickFirstAnchor,
  closest: closest,
  sibling: sibling,
  normalizeEvent: normalizeEvent,
  listeners: createListeners,
  root: function() {
    return get(this, 'doc').documentElement;
  },
  // TODO: Should I change these to use the standard names
  // even though they don't have a standard signature (querySelector*)
  elementById: function(id) {
    return get(this, 'doc').getElementById(id);
  },
  elementsByTagName: function(name, context) {
    context = typeof context === 'undefined' ? get(this, 'doc') : context;
    return context.getElementsByTagName(name);
  },
  elements: function(selector, context) {
    // don't ever be tempted to [...$$()] here
    // it should return a NodeList
    return $$(selector, context);
  },
  element: function(selector, context) {
    if (selector.substr(0, 1) === '#') {
      return this.elementById(selector.substr(1));
    }
    // TODO: This can just use querySelector
    return [...$$(selector, context)][0];
  },
  // ember components aren't strictly 'dom-like'
  // but if you think of them as a web component 'shim'
  // then it makes more sense to think of them as part of the dom
  // with traditional/standard web components you wouldn't actually need this
  // method as you could just get to their methods from the dom element
  component: function(selector, context) {
    if (typeof selector !== 'string') {
      return $_(selector);
    }
    return $_(this.element(selector, context));
  },
  components: function(selector, context) {
    return [...this.elements(selector, context)]
      .map(function(item) {
        return $_(item);
      })
      .filter(function(item) {
        return item != null;
      });
  },
});
