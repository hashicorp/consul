import Adapter, {
  REQUEST_CREATE,
  REQUEST_UPDATE,
  DATACENTER_QUERY_PARAM as API_DATACENTER_KEY,
} from './application';

import { get } from '@ember/object';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/role';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { OK as HTTP_OK } from 'consul-ui/utils/http/status';
import { PUT as HTTP_PUT } from 'consul-ui/utils/http/method';
import minimizeModel from 'consul-ui/utils/minimizeModel';

const createTemplatedPolicies = function(item) {
  item.ServiceIdentities.forEach(function(identity) {
    const policy = {
      Name: identity.ServiceName,
      template: 'service-identity',
      Datacenters: identity.Datacenters,
    };
    item.Policies.push(policy);
  });
  return item;
};
const createServiceIdentities = function(item) {
  item.ServiceIdentities = item.Policies.filter(function(item) {
    return get(item, 'template') === 'service-identity';
  }).map(function(item) {
    return {
      ServiceName: get(item, 'Name'),
      Datacenters: get(item, 'Datacenters'),
    };
  });
  item.Policies = item.Policies.filter(function(item) {
    return get(item, 'template') !== 'service-identity';
  }).map(function(item) {
    return {
      Name: get(item, 'Name'),
      ID: get(item, 'ID'),
    };
  });
  return item;
};
export default Adapter.extend({
  urlForQuery: function(query, modelName) {
    return this.appendURL('acl/roles', [], this.cleanQuery(query));
  },
  urlForQueryRecord: function(query, modelName) {
    if (typeof query.id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return this.appendURL('acl/role', [query.id], this.cleanQuery(query));
  },
  urlForCreateRecord: function(modelName, snapshot) {
    return this.appendURL('acl/role', [], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForUpdateRecord: function(id, modelName, snapshot) {
    return this.appendURL('acl/role', [snapshot.attr(SLUG_KEY)], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  urlForDeleteRecord: function(id, modelName, snapshot) {
    return this.appendURL('acl/role', [snapshot.attr(SLUG_KEY)], {
      [API_DATACENTER_KEY]: snapshot.attr(DATACENTER_KEY),
    });
  },
  handleResponse: function(status, headers, payload, requestData) {
    let response = payload;
    if (status === HTTP_OK) {
      const url = this.parseURL(requestData.url);
      switch (true) {
        case response === true:
          response = this.handleBooleanResponse(url, response, PRIMARY_KEY, SLUG_KEY);
          break;
        case Array.isArray(response):
          response = this.handleBatchResponse(url, response, PRIMARY_KEY, SLUG_KEY);
          break;
        default:
          response = this.handleSingleResponse(url, response, PRIMARY_KEY, SLUG_KEY);
      }
    }
    return this._super(status, headers, response, requestData);
  },
  handleSingleResponse: function(url, response, primary, slug) {
    // Sometimes we get `Policies: null`, make null equal an empty array
    ['Policies', 'ServiceIdentities'].forEach(function(prop) {
      if (typeof response[prop] === 'undefined' || response[prop] === null) {
        response[prop] = [];
      }
    });
    return this._super(url, createTemplatedPolicies(response), primary, slug);
  },
  methodForRequest: function(params) {
    switch (params.requestType) {
      case REQUEST_CREATE:
        return HTTP_PUT;
    }
    return this._super(...arguments);
  },
  dataForRequest: function(params) {
    let data = this._super(...arguments);
    switch (params.requestType) {
      case REQUEST_UPDATE:
      case REQUEST_CREATE:
        data = createServiceIdentities(data.role);
        data.Policies = minimizeModel(data.Policies);
        break;
    }
    return data;
  },
});
