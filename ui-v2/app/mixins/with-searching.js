import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import WithListeners from 'consul-ui/mixins/with-listeners';
/**
 * WithSearching mostly depends on a `searchParams` object which must be set
 * inside the `init` function. The naming and usage of this is modelled on
 * `queryParams` but in contrast cannot _yet_ be 'hung' of the Controller
 * object, it MUST be set in the `init` method.
 * Reasons: As well as producing a eslint error, it can also be 'shared' amongst
 * child Classes of the component. It is not clear _yet_ whether mixing this in
 * avoids this and is something to be looked at in future to slightly improve DX
 * Please also see:
 * https://emberjs.com/api/ember/2.12/classes/Ember.Object/properties?anchor=mergedProperties
 *
 */
export default Mixin.create(WithListeners, {
  builder: service('search'),
  init: function() {
    this._super(...arguments);
    const params = this.searchParams || {};
    this.searchables = {};
    Object.keys(params).forEach(type => {
      const key = params[type];
      this.searchables[type] = get(this, 'builder').searchable(type);
      this.listen(this.searchables[type], 'change', e => {
        const value = e.target.value;
        set(this, key, value === '' ? null : value);
      });
    });
  },
});
