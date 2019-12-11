import RepositoryService from 'consul-ui/services/repository';
import { Promise } from 'rsvp';

const modelName = 'nspace';
const DEFAULT_NSPACE = 'default';
export default RepositoryService.extend({
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
