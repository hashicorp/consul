import ApplicationAdapter from './application';
import { typeOf } from '@ember/utils';
export default ApplicationAdapter.extend({
  urlForQuery(query, modelName) {
    let keys = '';
    if (typeOf(query.keys) !== 'undefined') {
      keys = '?keys';
      delete query.keys;
    }
    let key = '/';
    if (query.key !== key) {
      key += query.key;
    }
    delete query.key;
    return `/${this.namespace}/kv${key}${keys}`;
  },
});
