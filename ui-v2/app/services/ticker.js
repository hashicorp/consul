import Service from '@ember/service';
import { Tween } from 'consul-ui/utils/ticker';

let map;
export default Service.extend({
  init: function() {
    this._super(...arguments);
    this.reset();
  },
  tweenTo: function(props, obj = '', frames, method) {
    // TODO: Right now we only support string id's
    // but potentially look at allowing passing of other objects
    // especially DOM elements
    const id = obj;
    if (!map.has(id)) {
      map.set(id, props);
      return props;
    } else {
      obj = map.get(id);
      if (obj instanceof Tween) {
        obj = obj.stop().getTarget();
      }
      map.set(id, Tween.to(obj, props, frames, method));
      return obj;
    }
  },
  // TODO: We'll try and use obj later for ticker bookkeeping
  destroy: function(obj) {
    this.reset();
    return Tween.destroy();
  },
  reset: function() {
    map = new Map();
  },
});
