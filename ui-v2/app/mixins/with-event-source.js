import Mixin from '@ember/object/mixin';
import { set } from '@ember/object';
import { computed as catchable } from 'consul-ui/computed/catchable';
import purify from 'consul-ui/utils/computed/purify';

import WithListeners from 'consul-ui/mixins/with-listeners';
const PREFIX = '_';
export default Mixin.create(WithListeners, {
  setProperties: function(model) {
    const _model = {};
    Object.keys(model).forEach(prop => {
      // here (see comment below on deleting)
      if (model[prop] && typeof model[prop].addEventListener === 'function') {
        let meta;
        // TODO: metaForProperty throws an error if the property is not
        // computed-like, this is far from ideal but happy with this
        // until we can find a better way in an ember post 2.18 world
        // of finding out if a property is computed or not
        // (or until we switch all this out for <DataSource /> compoments
        try {
          meta = this.constructor.metaForProperty(prop);
        } catch (e) {
          meta = {};
        }
        if (typeof meta.catch === 'function') {
          _model[`${PREFIX}${prop}`] = model[prop];
          this.listen(_model[`_${prop}`], 'error', meta.catch.bind(this));
        } else {
          _model[prop] = model[prop];
        }
      } else {
        _model[prop] = model[prop];
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

          // ember 3 upgrade extra note:
          // in an ember 2.18 world this used to be `delete this[prop]`
          // to entirely reset everything instead of nulling out
          // at some point after 2.18 this delete seems to cause problems
          // probably due how Proxies work.
          // whilst setting something to 'no set' usually feels a bit strange
          // its the closest we can get to a `delete` here, unless ember has
          // some special way of delete things entirely
          // null also works as the logic related to this is just a falsey check
          // but setting something to undefined is closer to delete

          set(this, prop, undefined);
        }
      });
    }
    return this._super(...arguments);
  },
});
export const listen = purify(catchable, function(props) {
  return props.map(item => `${PREFIX}${item}`);
});
