import Adapter, {
  REQUEST_CREATE,
  REQUEST_UPDATE,
  DATACENTER_QUERY_PARAM as API_DATACENTER_KEY,
} from './application';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/intention';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';
import { POST as HTTP_POST } from 'consul-ui/utils/http/method';
import makeAttrable from 'consul-ui/utils/makeAttrable';
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('connect/intentions', [], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
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
  isUpdateRecord: function(url, method) {
    return (
      url.pathname ===
      this.parseURL(
        this.urlForUpdateRecord(null, 'intention', makeAttrable({ [DATACENTER_KEY]: '' }))
      ).pathname
    );
  },
  isCreateRecord: function(url, method) {
    return (
      method.toUpperCase() === HTTP_POST &&
      url.pathname ===
        this.parseURL(this.urlForCreateRecord('intention', makeAttrable({ [DATACENTER_KEY]: '' })))
          .pathname
    );
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    const method = requestData.method;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case this.isQueryRecord(url, method):
        case this.isUpdateRecord(url, method):
        case this.isCreateRecord(url, method):
          // TODO: We just need to upgrade this (^^ sorry linter) entire API to
          // use a full request-like object
          response = this.handleSingleResponse(url, response, PRIMARY_KEY, SLUG_KEY);
          break;
        default:
          response = this.handleBatchResponse(url, response, PRIMARY_KEY, SLUG_KEY);
      }
    }
    return this._super(status, headers, response, requestData);
  },
  dataForRequest: function(params) {
    const data = this._super(...arguments);
    switch (params.requestType) {
      case REQUEST_UPDATE:
      case REQUEST_CREATE:
        return data.intention;
    }
    return data;
  },
});
