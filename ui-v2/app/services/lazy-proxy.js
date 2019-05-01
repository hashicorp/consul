import Service from '@ember/service';
import { get } from '@ember/object';

export default Service.extend({
  shouldProxy: function(content, method) {
    return false;
  },
  init: function() {
    this._super(...arguments);
    const content = get(this, 'content');
    for (let prop in content) {
      if (typeof content[prop] === 'function') {
        if (this.shouldProxy(content, prop)) {
          this[prop] = function() {
            const cb = this.execute(content, prop);
            if (typeof cb.then !== 'undefined') {
              return cb.then(method => {
                return method.apply(this, arguments);
              });
            } else {
              return cb.apply(this, arguments);
            }
          };
        } else if (typeof this[prop] !== 'function') {
          this[prop] = function() {
            return content[prop](...arguments);
          };
        }
      }
    }
  },
});
