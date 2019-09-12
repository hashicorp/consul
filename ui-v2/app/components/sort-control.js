import Component from '@ember/component';
import { get, set } from '@ember/object';
export default Component.extend({
  classNames: ['sort-control'],
  direction: 'asc',
  onchange: function() {},
  actions: {
    change: function(e) {
      if (e.target.type === 'checkbox') {
        set(this, 'direction', e.target.checked ? 'desc' : 'asc');
      }
      this.onchange({ target: { value: `${get(this, 'value')}:${get(this, 'direction')}` } });
    },
  },
});
