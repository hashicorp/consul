import Adapter from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/node';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('internal/ui/nodes', [], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
    return this.appendURL('internal/ui/node', [query.id], this.cleanQuery(query));
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case this.isQueryRecord(url):
          response = {
            ...response,
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
