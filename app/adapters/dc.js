import Adapter from './application';

export default Adapter.extend({
  urlForFindAll: function() {
    return `/${this.namespace}/catalog/datacenters`;
  },
});
