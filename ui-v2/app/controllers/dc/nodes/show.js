import Controller from '@ember/controller';
import { get } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';

export default Controller.extend(WithFiltering, {
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  filter: function(item, { s = '' }) {
    return (
      get(item, 'Service')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0
    );
  },
});
