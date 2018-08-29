import Adapter, { DATACENTER_QUERY_PARAM as API_DATACENTER_KEY } from './application';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/node';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';
// TODO: Looks like ID just isn't used at all
// consider just using .Node for the SLUG_KEY
const fillSlug = function(item) {
  if (item[SLUG_KEY] === '') {
    item[SLUG_KEY] = item['Node'];
  }
  return item;
};
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('internal/ui/nodes', [], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return this.appendURL('internal/ui/node', [query.id], this.cleanQuery(query));
  },
  handleBatchResponse: function(url, response, primary, slug) {
    const dc = url.searchParams.get(API_DATACENTER_KEY) || '';
    return response.map((item, i, arr) => {
      // this can go in the serializer
      item = fillSlug(item);
      // this could be replaced by handleSingleResponse
      // maybe perf test first although even polyfilled searchParams should be super fast
      return {
        ...item,
        ...{
          [DATACENTER_KEY]: dc,
          [PRIMARY_KEY]: this.uidForURL(url, item[SLUG_KEY]),
        },
      };
    });
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    const method = requestData.method;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case this.isQueryRecord(url, method):
          response = this.handleSingleResponse(url, fillSlug(response), PRIMARY_KEY, SLUG_KEY);
          break;
        default:
          response = this.handleBatchResponse(url, response, PRIMARY_KEY, SLUG_KEY);
      }
    }
    return this._super(status, headers, response, requestData);
  },
});
