import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import WithListeners from 'consul-ui/mixins/with-listeners';

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
