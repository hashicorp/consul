import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  classNames: ['tab-section'],
  'data-test-radiobutton': computed('name,id', function() {
    return `${this.get('name')}_${this.get('id')}`;
  }),
  name: 'tab',
  onchange: function() {},
});
