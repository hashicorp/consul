import Adapter from './application';
export default Adapter.extend({
  urlForFindAll() {
    return `/${this.namespace}/internal/ui/nodes`;
  },
  urlForFindRecord(id, modelName) {
    return `/${this.namespace}/internal/ui/node/${id}`;
  },
});
