import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import { Promise } from 'rsvp';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/token';
import statusFactory from 'consul-ui/utils/acls-status';
import isValidServerErrorFactory from 'consul-ui/utils/http/acl/is-valid-server-error';

const isValidServerError = isValidServerErrorFactory();
const status = statusFactory(isValidServerError, Promise);
const MODEL_NAME = 'token';

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
    return get(this, 'store')
      .self(this.getModelName(), {
        secret: secret,
        dc: dc,
      })
      .catch(e => {
        // If we get this 500 RPC error, it means we are a legacy ACL cluster
        // set AccessorID to null - which for the frontend means legacy mode
        if (isValidServerError(e)) {
          return {
            AccessorID: null,
            SecretID: secret,
          };
        }
        return Promise.reject(e);
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
  findByRole: function(id, dc) {
    return get(this, 'store').query(this.getModelName(), {
      role: id,
      dc: dc,
    });
  },
});
