import Mixin from '@ember/object/mixin';
import WithFiltering from 'consul-ui/mixins/with-filtering';

export default Mixin.create(WithFiltering, {
  queryParams: {
    status: {
      as: 'status',
    },
    s: {
      as: 'filter',
    },
  },
});
