import Adapter from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/proxy';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
    // https://www.consul.io/api/catalog.html#list-nodes-for-connect-capable-service
    return this.appendURL('catalog/connect', [query.id], this.cleanQuery(query));
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      response = this.handleBatchResponse(url, response, PRIMARY_KEY, SLUG_KEY);
    }
    return this._super(status, headers, response, requestData);
  },
});
