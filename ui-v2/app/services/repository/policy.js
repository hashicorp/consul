import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import { Promise } from 'rsvp';
import statusFactory from 'consul-ui/utils/acls-status';
import isValidServerErrorFactory from 'consul-ui/utils/http/acl/is-valid-server-error';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/policy';

const isValidServerError = isValidServerErrorFactory();
const status = statusFactory(isValidServerError, Promise);
const MODEL_NAME = 'policy';

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
  persist: function(item) {
    // only if a policy doesn't have a template, save it
    // right now only ServiceIdentities have templates and
    // are not saveable themselves (but can be saved to a Role/Token)
    switch (get(item, 'template')) {
      case '':
        return item.save();
    }
    return Promise.resolve(item);
  },
  translate: function(item) {
    return get(this, 'store').translate('policy', get(item, 'Rules'));
  },
});
