import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { fromPromise } from 'consul-ui/utils/dom/event-source';

export default Component.extend({
  repo: service('repository/oidc-provider'),
  dom: service('dom'),
  tagName: '',
  onchange: function(e) {},
  onerror: function(e) {},
  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this.repo.close();
    this._listeners.remove();
  },
  didInsertElement: function() {
    if (this.source) {
      this.source.close();
    }
    // TODO: Could this use once? Double check but I don't think it can
    this.source = fromPromise(this.repo.findCodeByURL(this.src));
    this._listeners.add(this.source, {
      message: e => this.onchange(e),
      error: e => this.onerror(e),
    });
  },
});
