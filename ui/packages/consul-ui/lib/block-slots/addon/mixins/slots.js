import { computed, get } from '@ember/object';
import { A } from '@ember/array';
import Mixin from '@ember/object/mixin';
export default Mixin.create({
  _slots: computed(function() {
    return A();
  }),
  _activateSlot(name) {
    get(this, '_slots').addObject(name);
  },
  _deactivateSlot(name) {
    get(this, '_slots').removeObject(name);
  },
  _isRegistered(name) {
    return get(this, '_slots').includes(name);
  },
});
