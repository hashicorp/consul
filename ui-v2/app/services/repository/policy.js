import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import { Promise } from 'rsvp';
import statusFactory from 'consul-ui/utils/acls-status';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/policy';
const MODEL_NAME = 'policy';
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
  translate: function(item) {
    return get(this, 'store').translate('policy', get(item, 'Rules'));
  },
});
