import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';

export default Controller.extend(WithFiltering, {
  init: function() {
    this._super(...arguments);
    this.tabs = ['Health Checks', 'Services', 'Round Trip Time', 'Lock Sessions'];
    this.selectedTab = 'health-checks';
  },
  filter: function(item, { s = '', status = '' }) {
    return (
      get(item, 'Service')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0
    );
  },
  actions: {
    change: function(event) {
      set(this, 'selectedTab', event.target.value);
    },
  },
});
