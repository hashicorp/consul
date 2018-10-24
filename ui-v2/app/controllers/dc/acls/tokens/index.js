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
  filter: function(item, { s = '', type = '' }) {
    const sLower = s.toLowerCase();
    return (
      get(item, 'AccessorID')
        .toLowerCase()
        .indexOf(sLower) !== -1 ||
      get(item, 'Name')
        .toLowerCase()
        .indexOf(sLower) !== -1 ||
      get(item, 'Description')
        .toLowerCase()
        .indexOf(sLower) !== -1 ||
      (get(item, 'Policies') || []).some(function(item) {
        return item.Name.toLowerCase().indexOf(sLower) !== -1;
      })
    );
  },
  actions: {
    sendClone: function(item) {
      this.send('clone', item);
    },
  },
});
