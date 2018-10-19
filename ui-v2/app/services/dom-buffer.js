import Service from '@ember/service';
import Evented from '@ember/object/evented';
const buffer = {};
export default Service.extend(Evented, {
  // TODO: Consider renaming this and/or
  // `delete`ing the buffer (but not the DOM element)
  // flush should flush, but maybe being able to re-flush
  // after you've flushed could be handy
  flush: function(name) {
    return buffer[name];
  },
  add: function(name, dom) {
    this.trigger('add', dom);
    buffer[name] = dom;
    return dom;
  },
  remove: function(name) {
    buffer[name].remove();
    delete buffer[name];
  },
});
