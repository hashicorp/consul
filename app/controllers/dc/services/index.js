import Controller from '@ember/controller';
import { get } from '@ember/object';
import WithHealthFiltering from 'consul-ui/mixins/with-health-filtering';
export default Controller.extend(WithHealthFiltering, {
  filter: function(item, { s = '', status = '' }) {
    return (
      get(item, 'Name')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0 && item.hasStatus(status)
    );
  },
});
