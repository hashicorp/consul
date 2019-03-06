import Adapter from './application';
export default Adapter.extend({
  requestForQuery: function(request, { dc, index, id }) {
    return request`
      GET /v1/internal/ui/nodes?${{ dc, index }}
    `;
  },
  requestForQueryRecord: function(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/internal/ui/node/${id}?${{ dc, index }}
    `;
  },
  // keep the none queryLeader bits in here ready to be refactored
  // they will all be noops until these are added to the refactor
  urlForRequest: function({ type, snapshot, requestType }) {
    switch (requestType) {
      case 'queryLeader':
        return this.urlForQueryLeader(snapshot, type.modelName);
    }
    return this._super(...arguments);
  },
  urlForQueryLeader: function(query, modelName) {
    // https://www.consul.io/api/status.html#get-raft-leader
    return this.appendURL('status/leader', [], this.cleanQuery(query));
  },
  isQueryLeader: function(url, method) {
    return url.pathname === this.parseURL(this.urlForQueryLeader({})).pathname;
  },
  queryLeader: function(store, modelClass, id, snapshot) {
    const params = {
      store: store,
      type: modelClass,
      id: id,
      snapshot: snapshot,
      requestType: 'queryLeader',
    };
    // _requestFor is private... but these methods aren't, until they disappear..
    const request = {
      method: this.methodForRequest(params),
      url: this.urlForRequest(params),
      headers: this.headersForRequest(params),
      data: this.dataForRequest(params),
    };
    // TODO: private..
    return this._makeRequest(request);
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
      let temp, port, address;
      switch (true) {
        case this.isQueryLeader(url, method):
          // This response is just an ip:port like `"10.0.0.1:8000"`
          // split it and make it look like a `C`onsul.`R`esponse
          // popping off the end for ports should cover us for IPv6 addresses
          // as we should always get a `address:port` or `[a:dd:re:ss]:port` combo
          temp = response.split(':');
          port = temp.pop();
          address = temp.join(':');
          response = {
            Address: address,
            Port: port,
          };
          break;
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
