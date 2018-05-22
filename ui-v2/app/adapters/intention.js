import Adapter, { DATACENTER_KEY as API_DATACENTER_KEY } from './application';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/intention';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';
import makeAttrable from 'consul-ui/utils/makeAttrable';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('connect/intentions', [], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
    return this.appendURL('connect/intentions', [query.id], this.cleanQuery(query));
  },
  urlForCreateRecord: function(modelName, snapshot) {
    return this.appendURL('connect/intentions', [], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForUpdateRecord: function(id, modelName, snapshot) {
    return this.appendURL('connect/intentions', [snapshot.attr(SLUG_KEY)], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    return this.appendURL('connect/intentions', [snapshot.attr(SLUG_KEY)], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  isUpdateRecord: function(url) {
    return (
      url.pathname ===
      this.parseURL(
        this.urlForUpdateRecord(null, 'intention', makeAttrable({ [DATACENTER_KEY]: '' }))
      ).pathname
    );
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case this.isQueryRecord(url):
        case this.isUpdateRecord(url):
          // case this.isCreateRecord(url):
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
