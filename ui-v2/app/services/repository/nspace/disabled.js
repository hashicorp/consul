import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';

const modelName = 'nspace';
const DEFAULT_NSPACE = 'default';
export default RepositoryService.extend({
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  getSlugKey: function() {
    return SLUG_KEY;
  },
  getModelName: function() {
    return modelName;
  },
  findAll: function(configuration = {}) {
    return Promise.resolve([]);
  },
  getActive: function() {
    return {
      Name: DEFAULT_NSPACE,
    };
  },
  authorize: function(dc, nspace) {
    return Promise.resolve([]);
  },
});
