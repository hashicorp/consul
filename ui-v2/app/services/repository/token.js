import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import { Promise } from 'rsvp';
import statusFactory from 'consul-ui/utils/acls-status';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/token';
const MODEL_NAME = 'token';
const status = statusFactory(Promise);
export default RepositoryService.extend({
  getModelName: function() {
    return MODEL_NAME;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  getSlugKey: function() {
    return SLUG_KEY;
  },
  status: function(obj) {
    return status(obj);
  },
  self: function(secret, dc) {
    return get(this, 'store').self(this.getModelName(), {
      secret: secret,
      dc: dc,
    });
  },
  clone: function(item) {
    return get(this, 'store').clone(this.getModelName(), get(item, PRIMARY_KEY));
  },
  findByPolicy: function(id, dc) {
    return get(this, 'store').query(this.getModelName(), {
      policy: id,
      dc: dc,
    });
  },
});
