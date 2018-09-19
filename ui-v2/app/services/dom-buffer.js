import Service from '@ember/service';
import Evented from '@ember/object/evented';
const buffer = {};
export default Service.extend(Evented, {
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
