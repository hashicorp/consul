import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  // Whether or not the dropdown menu can be seen
  isDropdownVisible: false,
  // Returns the total number of failing checks.
  // We treat any non-passing checks as failing
  totalChecksFailing: function() {
    return this.get('nodes').reduce(function(sum, node) {
      return sum + node.get('failingChecks');
    }, 0);
  }.property('nodes'),
  totalChecksPassing: function() {
    return this.get('nodes').reduce(function(sum, node) {
      return sum + node.get('passingChecks');
    }, 0);
  }.property('nodes'),
  //
  // Returns the human formatted message for the button state
  //
  checkMessage: function() {
    var failingChecks = this.get('totalChecksFailing');
    var passingChecks = this.get('totalChecksPassing');
    if (this.get('hasFailingChecks') === true) {
      return failingChecks + ' failing';
    } else {
      return passingChecks + ' passing';
    }
  }.property('nodes'),
  checkStatus: function() {
    if (this.get('hasFailingChecks') === true) {
      return 'failing';
    } else {
      return 'passing';
    }
  }.property('nodes'),
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
