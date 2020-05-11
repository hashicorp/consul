import Service from '@ember/service';
import Evented from '@ember/object/evented';
let buffers;
// TODO: This all should be replaced with {{#in-element}} if possible
export default Service.extend(Evented, {
  init: function() {
    this._super(...arguments);
    buffers = {};
  },
  willDestroy: function() {
    Object.entries(buffers).forEach(function([key, items]) {
      items.forEach(function(item) {
        item.remove();
      });
    });
    buffers = null;
  },
  // TODO: Consider renaming this and/or
  // `delete`ing the buffer (but not the DOM element)
  // flush should flush, but maybe being able to re-flush
  // after you've flushed could be handy
  flush: function(name) {
    return buffers[name];
  },
  add: function(name, dom) {
    this.trigger('add', dom);
    if (typeof buffers[name] === 'undefined') {
      buffers[name] = [];
    }
    buffers[name].push(dom);
    return dom;
  },
  remove: function(name, dom) {
    if (typeof buffers[name] !== 'undefined') {
      const buffer = buffers[name];
      const i = buffer.findIndex(item => item === dom);
      if (i !== -1) {
        const item = buffer.splice(i, 1)[0];
        item.remove();
      }
      if (buffer.length === 0) {
        delete buffers[name];
      }
    }
  },
});
