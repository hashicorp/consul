import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY } from 'consul-ui/models/intention';
const modelName = 'intention';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  // findByService: function(slug, dc, configuration = {}) {
  //   const query = {
  //     dc: dc,
  //     id: slug,
  //   };
  //   if (typeof configuration.cursor !== 'undefined') {
  //     query.index = configuration.cursor;
  //   }
  //   return get(this, 'store').queryRecord(this.getModelName(), query);
  // }
});
