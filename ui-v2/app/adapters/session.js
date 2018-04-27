import Adapter, { REQUEST_DELETE, DATACENTER_KEY as API_DATACENTER_KEY } from './application';

import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/session';
import { PUT as HTTP_PUT } from 'consul-ui/utils/http/method';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';

export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('session/node', [query.id], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
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
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case response === true:
          response = {
            [PRIMARY_KEY]: this.uidForURL(url),
          };
          break;
        case this.isQueryRecord(url):
          response = {
            ...response[0],
            ...{
              [PRIMARY_KEY]: this.uidForURL(url),
            },
          };
          break;
        default:
          response = response.map((item, i, arr) => {
            return {
              ...item,
              ...{
                [PRIMARY_KEY]: this.uidForURL(url, item[SLUG_KEY]),
              },
            };
          });
      }
    }
    return this._super(status, headers, response, requestData);
  },
});
