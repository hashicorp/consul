import Adapter from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/service';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';
const URL_PREFIX_SINGLE = 'health/service';
const URL_PREFIX_MULTIPLE = 'internal/ui/services';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL(URL_PREFIX_MULTIPLE, [], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return this.appendURL(URL_PREFIX_SINGLE, [query.id], this.cleanQuery(query));
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    const method = requestData.method;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case this.isQueryRecord(url, method):
          response = this.handleSingleResponse(url, { Nodes: response }, PRIMARY_KEY, SLUG_KEY);
          break;
        default:
          response = this.handleBatchResponse(url, response, PRIMARY_KEY, SLUG_KEY);
      }
    }
    return this._super(status, headers, response, requestData);
  },
});
