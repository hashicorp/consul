import Component from '@ember/component';
import { get, set } from '@ember/object';
import SlotsMixin from 'block-slots';
import WithListeners from 'consul-ui/mixins/with-listeners';

export default Component.extend(WithListeners, SlotsMixin, {
  tagName: '',
  didReceiveAttrs: function() {
    this._super(...arguments);
    this.removeListeners();
    const dispatcher = get(this, 'dispatcher');
    if (dispatcher) {
      this.listen(dispatcher, 'change', e => {
        set(this, 'items', e.target.data);
      });
      set(this, 'items', get(dispatcher, 'data'));
    }
  },
});
