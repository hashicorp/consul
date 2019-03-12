import Mixin from '@ember/object/mixin';
import { computed as catchable } from 'consul-ui/computed/catchable';
import purify from 'consul-ui/utils/computed/purify';

import WithListeners from 'consul-ui/mixins/with-listeners';
const PREFIX = '_';
export default Mixin.create(WithListeners, {
  setProperties: function(model) {
    const _model = {};
    Object.keys(model).forEach(key => {
      // here (see comment below on deleting)
      if (typeof this[key] !== 'undefined' && this[key].isDescriptor) {
        _model[`${PREFIX}${key}`] = model[key];
        const meta = this.constructor.metaForProperty(key) || {};
        if (typeof meta.catch === 'function') {
          if (typeof _model[`${PREFIX}${key}`].addEventListener === 'function') {
            this.listen(_model[`_${key}`], 'error', meta.catch.bind(this));
          }
        }
      } else {
        _model[key] = model[key];
      }
    });
    return this._super(_model);
  },
  reset: function(exiting) {
    if (exiting) {
      Object.keys(this).forEach(prop => {
        if (this[prop] && typeof this[prop].close === 'function') {
          this[prop].close();
          // ember doesn't delete on 'resetController' by default
          // right now we only call reset when we are exiting, therefore a full
          // setProperties will be called the next time we enter the Route so this
          // is ok for what we need and means that the above conditional works
          // as expected (see 'here' comment above)
          delete this[prop];
        }
      });
    }
    return this._super(...arguments);
  },
});
export const listen = purify(catchable, function(props) {
  return props.map(item => `${PREFIX}${item}`);
});
