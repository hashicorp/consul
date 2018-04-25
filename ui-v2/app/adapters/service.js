import Adapter from './application';
const PRIMARY_KEY = 'Id';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('internal/ui/services');
  },
  urlForQueryRecord: function(query, modelName) {
    const id = query.id;
    delete query.id;
    return this.appendURL('health/service', [id]);
  },
  isQueryRecord: function(parts) {
    const url = parts
      .slice(0, -1)
      .concat([''])
      .join('/');
    return this.urlForQueryRecord({ id: '' }) === url;
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    if (status === 200) {
      // here status is a number..
      const parts = requestData.url.split('/');
      if (this.isQueryRecord(parts)) {
        response = {
          [PRIMARY_KEY]: parts.pop(),
          Nodes: response,
        };
      } else {
        // isQuery
        response = response.map(function(item, i, arr) {
          return {
            ...item,
            ...{
              [PRIMARY_KEY]: item.Name,
            },
          };
        });
      }
    }
    return this._super(status, headers, response, requestData);
    // return this._super(status, headers, {services: response}, requestData);
  },
});
