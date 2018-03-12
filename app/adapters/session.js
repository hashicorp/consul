import Adapter from './application';

export default Adapter.extend({
  urlForQuery(query, modelName) {
    const id = query.node;
    delete query.node;
    return `/${this.namespace}/session/node/${id}`;
  },
});
