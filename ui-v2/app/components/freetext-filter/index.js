import Component from '@ember/component';
const ENTER = 13;
const proxyEventTargetValue = function(e, cb) {
  const target = e.target;
  return new Proxy(e, {
    get: function(obj, prop, receiver) {
      if (prop === 'target') {
        return new Proxy(target, {
          get: function(obj, prop, receiver) {
            if (prop === 'value') {
              return cb(e);
            }
            return target[prop];
          },
        });
      }
      return Reflect.get(...arguments);
    },
  });
};
export default Component.extend({
  tagName: '',
  search: function(e) {
    if (typeof this.onsearch !== 'undefined') {
      this.onsearch(e);
    } else {
      let searchable = this.searchable;
      if (!Array.isArray(searchable)) {
        searchable = [searchable];
      }
      searchable.forEach(function(item) {
        item.search(e.target.value);
      });
    }
  },
  actions: {
    change: function(e) {
      if (e.target.value === '') {
        this.search(proxyEventTargetValue(e, () => undefined));
      } else {
        this.search(e);
      }
    },
    keydown: function(e) {
      if (e.keyCode === ENTER) {
        e.preventDefault();
      }
    },
  },
});
