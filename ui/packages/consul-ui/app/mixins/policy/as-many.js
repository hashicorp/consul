/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';

const normalizeIdentities = function (items, template, name, dc) {
  return (items || []).map(function (item) {
    const policy = {
      template: template,
      Name: item[name],
    };
    if (typeof item[dc] !== 'undefined') {
      policy[dc] = item[dc];
    }
    return policy;
  });
};
// Sometimes we get `Policies: null`, make null equal an empty array
// and add an empty template
const normalizePolicies = function (items) {
  return (items || []).map(function (item) {
    return {
      template: '',
      ...item,
    };
  });
};
const serializeIdentities = function (items, template, name, dc) {
  return items
    .filter(function (item) {
      return item.template === template;
    })
    .map(function (item) {
      const identity = {
        [name]: item.Name,
      };
      if (typeof get(item, dc) !== 'undefined') {
        identity[dc] = item[dc];
      }
      return identity;
    });
};
const serializePolicies = function (items) {
  return items
    .filter(function (item) {
      const template = get(item, 'template');
      return template === '' || typeof template === 'undefined';
    })
    .map(function (item) {
      // Extract ID and Name - use get() for both to handle models and objects
      return {
        ID: get(item, 'ID'),
        Name: get(item, 'Name'),
      };
    })
    .filter(function (item) {
      // Only include items that have both ID and Name
      return item.ID && item.Name;
    });
};

export default Mixin.create({
  //TODO: what about update and create?
  respondForQueryRecord: function (respond, query) {
    return this._super(function (cb) {
      return respond((headers, body) => {
        body.Policies = normalizePolicies(body.Policies)
          .concat(
            normalizeIdentities(
              body.ServiceIdentities,
              'service-identity',
              'ServiceName',
              'Datacenters'
            )
          )
          .concat(
            normalizeIdentities(body.NodeIdentities, 'node-identity', 'NodeName', 'Datacenter')
          );
        return cb(headers, body);
      });
    }, query);
  },
  respondForQuery: function (respond, query) {
    return this._super(function (cb) {
      return respond(function (headers, body) {
        return cb(
          headers,
          body.map(function (item) {
            item.Policies = normalizePolicies(item.Policies)
              .concat(
                normalizeIdentities(
                  item.ServiceIdentities,
                  'service-identity',
                  'ServiceName',
                  'Datacenters'
                )
              )
              .concat(
                normalizeIdentities(item.NodeIdentities, 'node-identity', 'NodeName', 'Datacenter')
              );
            return item;
          })
        );
      });
    }, query);
  },
  serialize: function (snapshot, options) {
    const data = this._super(...arguments);

    let policies = [];
    if (snapshot && snapshot.record && snapshot.record.Policies) {
      policies = snapshot.record.Policies.toArray
        ? snapshot.record.Policies.toArray()
        : snapshot.record.Policies;
    }

    if ((!policies || policies.length === 0) && data.Policies && Array.isArray(data.Policies)) {
      policies = data.Policies;
    }

    data.ServiceIdentities = serializeIdentities(
      policies,
      'service-identity',
      'ServiceName',
      'Datacenters'
    );
    data.NodeIdentities = serializeIdentities(policies, 'node-identity', 'NodeName', 'Datacenter');
    data.Policies = serializePolicies(policies);
    return data;
  },
});
