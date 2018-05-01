import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';

export default Controller.extend(WithFiltering, {
  filter: function(item, { s = '', status = '' }) {
    return (
      get(item, 'Service')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0
    );
  },
});
