import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';

export default Controller.extend(WithFiltering, {
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  setProperties: function() {
    this._super(...arguments);
    set(this, 'selectedTab', 'health-checks');
  },
  filter: function(item, { s = '' }) {
    return (
      get(item, 'Service')
        .toLowerCase()
        .indexOf(s.toLowerCase()) !== -1
    );
  },
});
