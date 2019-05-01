import { REQUEST_CREATE, REQUEST_UPDATE } from 'consul-ui/adapters/application';

import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';

import minimizeModel from 'consul-ui/utils/minimizeModel';

const normalizeServiceIdentities = function(items) {
  return (items || []).map(function(item) {
    const policy = {
      template: 'service-identity',
      Name: item.ServiceName,
    };
    if (typeof item.Datacenters !== 'undefined') {
      policy.Datacenters = item.Datacenters;
    }
    return policy;
  });
};
const normalizePolicies = function(items) {
  return (items || []).map(function(item) {
    return {
      template: '',
      ...item,
    };
  });
};
const serializeServiceIdentities = function(items) {
  return items
    .filter(function(item) {
      return get(item, 'template') === 'service-identity';
    })
    .map(function(item) {
      const identity = {
        ServiceName: get(item, 'Name'),
      };
      if (get(item, 'Datacenters')) {
        identity.Datacenters = get(item, 'Datacenters');
      }
      return identity;
    });
};
const serializePolicies = function(items) {
  return items.filter(function(item) {
    return get(item, 'template') === '';
  });
};

export default Mixin.create({
  handleSingleResponse: function(url, response, primary, slug) {
    response.Policies = normalizePolicies(response.Policies).concat(
      normalizeServiceIdentities(response.ServiceIdentities)
    );
    return this._super(url, response, primary, slug);
  },
  dataForRequest: function(params) {
    const data = this._super(...arguments);
    const name = params.type.modelName;
    switch (params.requestType) {
      case REQUEST_UPDATE:
      // falls through
      case REQUEST_CREATE:
        // ServiceIdentities serialization must happen first, or a copy taken
        data[name].ServiceIdentities = serializeServiceIdentities(data[name].Policies);
        data[name].Policies = minimizeModel(serializePolicies(data[name].Policies));
        break;
    }
    return data;
  },
});
