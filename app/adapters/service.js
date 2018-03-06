import Adapter from './application';
export default Adapter.extend({
  urlForQuery(/*query, modelName*/) {
    return this.urlForFindAll(); /* modelName, snapshot?? */
  },
  urlForFindAll(/*modelName, snapshot*/) {
    return `/${this.namespace}/internal/ui/services`;
  },
  urlForFindRecord(id, modelName) {
    return `/${this.namespace}/health/service/${id}`;
  },
});
