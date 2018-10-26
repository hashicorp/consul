import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import { PRIMARY_KEY } from 'consul-ui/models/acl';
const modelName = 'acl';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  clone: function(item) {
    return get(this, 'store').clone(this.getModelName(), get(item, this.getPrimaryKey()));
  },
});
