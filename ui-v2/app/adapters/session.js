import Adapter, {
  REQUEST_DELETE,
  DATACENTER_QUERY_PARAM as API_DATACENTER_KEY,
} from './application';

import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/session';
import { PUT as HTTP_PUT } from 'consul-ui/utils/http/method';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';

export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return this.appendURL('session/node', [query.id], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return this.appendURL('session/info', [query.id], this.cleanQuery(query));
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    const query = {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    };
    return this.appendURL('session/destroy', [snapshot.attr(SLUG_KEY)], query);
  },
  methodForRequest: function(params) {
    switch (params.requestType) {
      case REQUEST_DELETE:
        return HTTP_PUT;
    }
    return this._super(...arguments);
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    const method = requestData.method;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case response === true:
          response = this.handleBooleanResponse(url, response, PRIMARY_KEY, SLUG_KEY);
          break;
        case this.isQueryRecord(url, method):
          response = this.handleSingleResponse(url, response[0], PRIMARY_KEY, SLUG_KEY);
          break;
        default:
          response = this.handleBatchResponse(url, response, PRIMARY_KEY, SLUG_KEY);
      }
    }
    return this._super(status, headers, response, requestData);
  },
});
