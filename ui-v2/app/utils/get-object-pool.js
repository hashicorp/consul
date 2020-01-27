export default function(dispose = function() {}, max, objects = []) {
  return {
    acquire: function(obj, id) {
      // TODO: what should happen if an ID already exists
      // should we ignore and release both? Or prevent from acquiring? Or generate a unique ID?
      // what happens if we can't get an id via getId or .id?
      // could potentially use Set
      objects.push(obj);
      if (typeof max !== 'undefined') {
        if (objects.length > max) {
          return dispose(objects.shift());
        }
      }
      return id;
    },
    // release releases the obj from the pool but **doesn't** dispose it
    release: function(obj) {
      let index = -1;
      let id;
      if (typeof obj === 'string') {
        id = obj;
      } else {
        id = obj.id;
      }
      objects.forEach(function(item, i) {
        let itemId;
        if (typeof item.getId === 'function') {
          itemId = item.getId();
        } else {
          itemId = item.id;
        }
        if (itemId === id) {
          index = i;
        }
      });
      if (index !== -1) {
        return objects.splice(index, 1)[0];
      }
    },
    purge: function() {
      let obj;
      const objs = [];
      while ((obj = objects.shift())) {
        objs.push(dispose(obj));
      }
      return objs;
    },
    dispose: function(id) {
      return dispose(this.release(id));
    },
  };
}
