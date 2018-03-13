import Adapter from './application';
export default Adapter.extend({
  urlForQuery(query, modelName) {
    return `/${this.namespace}/acl/list`;
  },
  urlForQueryRecord(query, modelName) {
    const acl = query.acl;
    delete query.acl;
    return `/${this.namespace}/acl/info/${acl}`;
  },
});
