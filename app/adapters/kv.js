import ApplicationAdapter from './application';

export default ApplicationAdapter.extend({
  urlForQuery() {
    return `/${this.namespace}/kv`;
  },
});
