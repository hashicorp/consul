import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import Component from '@ember/component';
const append = function(content) {
  this.element.appendChild(content);
};
export default Component.extend({
  buffer: service('dom-buffer'),
  init: function() {
    this._super(...arguments);
    this.append = append.bind(this);
  },
  didInsertElement: function() {
    get(this, 'buffer').on('add', this.append);
  },
  didDestroyElement: function() {
    get(this, 'buffer').off('add', this.append);
  },
});
