import RepositoryService from 'consul-ui/services/repository';
import config from 'consul-ui/config/environment';
import { Promise } from 'rsvp';

const modelName = 'nspace';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  findAll: function(configuration = {}) {
    return Promise.resolve([]);
  },
  getUndefinedName: function() {
    return config.CONSUL_NSPACES_UNDEFINED_NAME;
  },
  getActive: function() {
    return {
      Name: this.getUndefinedName(),
    };
  },
  authorize: function(dc, nspace) {
    return Promise.resolve([]);
  },
});
