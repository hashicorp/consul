import { inject as service } from '@ember/service';
import Component from '@ember/component';
export default Component.extend({
  buffer: service('dom-buffer'),
  getBufferName: function() {
    // TODO: Right now we are only using this for the modal layer
    // moving forwards you'll be able to name your buffers
    return 'modal';
  },
  didInsertElement: function() {
    this._super(...arguments);
    this.buffer.add(this.getBufferName(), this.element);
  },
  didDestroyElement: function() {
    this._super(...arguments);
    this.buffer.remove(this.getBufferName());
  },
});
