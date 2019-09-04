import { inject as service } from '@ember/service';
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
    this._super(...arguments);
    this.buffer.on('add', this.append);
  },
  didDestroyElement: function() {
    this._super(...arguments);
    this.buffer.off('add', this.append);
  },
});
