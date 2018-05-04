import ApplicationAdapter from './application';

import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/coordinate';

import { OK as HTTP_OK } from 'consul-ui/utils/http/status';

export default ApplicationAdapter.extend({
  urlForQuery: function(query, modelName) {
    // https://www.consul.io/api/coordinate.html#read-lan-coordinates-for-all-nodes
    return this.appendURL('coordinate/nodes', [], this.cleanQuery(query));
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      response = response.map((item, i, arr) => {
        return {
          ...item,
          ...{
            [PRIMARY_KEY]: this.uidForURL(url, item[SLUG_KEY]),
          },
        };
      });
    }
    return this._super(status, headers, response, requestData);
  },
});
