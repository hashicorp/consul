import Controller from '@ember/controller';
import { get } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';
import rightTrim from 'consul-ui/utils/right-trim';
export default Controller.extend(WithFiltering, {
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  filter: function(item, { s = '' }) {
    const key = rightTrim(get(item, 'Key'), '/')
      .split('/')
      .pop();
    return key.toLowerCase().indexOf(s.toLowerCase()) !== -1;
  },
});
