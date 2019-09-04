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
// Sometimes we get `Policies: null`, make null equal an empty array
// and add an empty template
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
  //TODO: what about update and create?
  respondForQueryRecord: function(respond, query) {
    return this._super(function(cb) {
      return respond((headers, body) => {
        body.Policies = normalizePolicies(body.Policies).concat(
          normalizeServiceIdentities(body.ServiceIdentities)
        );
        return cb(headers, body);
      });
    }, query);
  },
  respondForQuery: function(respond, query) {
    return this._super(function(cb) {
      return respond(function(headers, body) {
        return cb(
          headers,
          body.map(function(item) {
            item.Policies = normalizePolicies(item.Policies).concat(
              normalizeServiceIdentities(item.ServiceIdentities)
            );
            return item;
          })
        );
      });
    }, query);
  },
  serialize: function(snapshot, options) {
    const data = this._super(...arguments);
    data.ServiceIdentities = serializeServiceIdentities(data.Policies);
    data.Policies = minimizeModel(serializePolicies(data.Policies));
    return data;
  },
});
