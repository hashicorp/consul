import Adapter from './application';
import { PRIMARY_KEY } from 'consul-ui/models/service';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('internal/ui/services', [], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
    return this.appendURL('health/service', [query.id], this.cleanQuery(query));
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case this.isQueryRecord(url):
          response = {
            [PRIMARY_KEY]: this.uidForURL(url),
            Nodes: response,
          };
          break;
        default:
          response = response.map((item, i, arr) => {
            return {
              ...item,
              ...{
                [PRIMARY_KEY]: this.uidForURL(url, item.Name),
              },
            };
          });
      }
    }
    return this._super(status, headers, response, requestData);
  },
});
