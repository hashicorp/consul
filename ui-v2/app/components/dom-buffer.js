import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import Component from '@ember/component';
export default Component.extend({
  buffer: service('dom-buffer'),
  getBufferName: function() {
    return 'modal';
  },
  didInsertElement: function() {
    get(this, 'buffer').add(this.getBufferName(), this.element);
  },
  didDestroyElement: function() {
    get(this, 'buffer').remove(this.getBufferName());
  },
});
