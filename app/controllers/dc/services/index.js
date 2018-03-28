import Controller from '@ember/controller';
import WithHealthFiltering from 'consul-ui/mixins/with-health-filtering';
import { get } from '@ember/object';
export default Controller.extend(WithHealthFiltering, {
  filter: function(item, { s = '', status = '' }) {
    return (
      get(item, 'Name')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0 && item.hasStatus(status)
    );
  },
});
