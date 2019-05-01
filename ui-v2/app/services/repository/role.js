import RepositoryService from 'consul-ui/services/repository';
import { Promise } from 'rsvp';
import statusFactory from 'consul-ui/utils/acls-status';
import isValidServerErrorFactory from 'consul-ui/utils/http/acl/is-valid-server-error';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/role';

const isValidServerError = isValidServerErrorFactory();
const status = statusFactory(isValidServerError, Promise);
const MODEL_NAME = 'role';

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
});
