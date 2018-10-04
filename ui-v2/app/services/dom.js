import Service from '@ember/service';
import { getOwner } from '@ember/application';

import qsaFactory from 'consul-ui/utils/dom/qsa-factory';
import getComponentFactory from 'consul-ui/utils/get-component-factory';

// ember-eslint doesn't like you using a single $ so use double
// use $_ for components
const $$ = qsaFactory();
let $_;

export default Service.extend({
  init: function() {
    $_ = getComponentFactory(getOwner(this));
  },
  element: function(selector, context) {
    return $$(selector, context);
  },
  // ember components aren't strictly 'dom-like'
  // but if you think of them as a web component 'shim'
  // then it makes more sense to think of them as part of the dom
  // with traditional/standard web components you wouldn't actually need this
  // method as you could just get to their methods from the dom element
  component: function(selector, context) {
    // TODO: support passing a dom element, when we need to do that
    return $_([...this.element(selector, context)][0]);
  },
});
