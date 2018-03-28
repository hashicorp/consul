import Controller from '@ember/controller';
import WithFiltering from 'consul-ui/mixins/with-filtering';
export default Controller.extend(WithFiltering, {
  filter: function(item, { s = '', type = '' }) {
    return (
      item
        .get('Name')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0 &&
      (type === '' || item.get('Type') === type)
    );
  },
});
