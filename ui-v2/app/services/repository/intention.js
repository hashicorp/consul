import { set, get } from '@ember/object';
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
  create: function(obj) {
    delete obj.Namespace;
    return this._super({
      Action: 'allow',
      ...obj,
    });
  },
  persist: function(obj) {
    return this._super(...arguments).then(res => {
      // if Action is set it means we are an l4 type intention
      // we don't delete these at a UI level incase the user
      // would like to switch backwards and forwards between
      // allow/deny/l7 in the forms, but once its been saved
      // to the backend we then delete them
      if (get(res, 'Action.length')) {
        set(res, 'Permissions', []);
      }
      return res;
    });
  },
  findByService: function(slug, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      nspace: nspace,
      filter: `SourceName == "${slug}" or DestinationName == "${slug}" or SourceName == "*" or DestinationName == "*"`,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), {
      ...query,
    });
  },
});
