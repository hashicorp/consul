import { REQUEST_CREATE, REQUEST_UPDATE } from 'consul-ui/adapters/application';

import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';

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
const createServiceIdentities = function(items) {
  return items
    .filter(function(item) {
      return get(item, 'template') === 'service-identity';
    })
    .map(function(item) {
      return {
        ServiceName: get(item, 'Name'),
        Datacenters: get(item, 'Datacenters'),
      };
    });
};
const createPolicies = function(items) {
  return items.filter(function(item) {
    return get(item, 'template') === '';
  });
};

export default Mixin.create({
  handleSingleResponse: function(url, response, primary, slug) {
    ['Policies', 'ServiceIdentities'].forEach(function(prop) {
      if (typeof response[prop] === 'undefined' || response[prop] === null) {
        response[prop] = [];
      }
    });
    return this._super(url, createTemplatedPolicies(response), primary, slug);
  },
  dataForRequest: function(params) {
    const data = this._super(...arguments);
    const name = params.type.modelName;
    switch (params.requestType) {
      case REQUEST_UPDATE:
      // falls through
      case REQUEST_CREATE:
        data[name].ServiceIdentities = createServiceIdentities(data[name].Policies);
        data[name].Policies = minimizeModel(createPolicies(data[name].Policies));
        break;
    }
    return data;
  },
});
