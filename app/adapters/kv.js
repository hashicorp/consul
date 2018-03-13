import ApplicationAdapter from './application';
const defaultPrefix = function(value, prefix = '/') {
  if (value !== prefix) {
    prefix += value;
  }
  return prefix;
};
export default ApplicationAdapter.extend({
  urlForQuery: function(query, modelName) {
    const key = defaultPrefix(query.key);
    delete query.key;
    return `/${this.namespace}/kv${key}?keys`;
  },
  urlForQueryRecord: function(query, modelName) {
    const key = defaultPrefix(query.key);
    delete query.key;
    return `/${this.namespace}/kv${key}`;
  },
});
