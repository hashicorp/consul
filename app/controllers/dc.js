import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  isDropdownVisible: false,
  totalChecksFailing: computed.sum('nodes.@each.failingChecks'),
  // totalChecksPassing: computed.sum(
  //   'nodes.@each.passingChecks'
  // ),
  hasFailingChecks: computed.gt('totalChecksFailing', 0),
  actions: {
    // Hide and show the dropdown menu
    toggle: function(item) {
      this.toggleProperty('isDropdownVisible');
    },
    // Just hide the dropdown menu
    hideDrop: function(item) {
      this.set('isDropdownVisible', false);
    },
  },
});
