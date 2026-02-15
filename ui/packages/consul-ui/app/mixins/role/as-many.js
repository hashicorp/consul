/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Mixin from '@ember/object/mixin';

import minimizeModel from 'consul-ui/utils/minimizeModel';

export default Mixin.create({
  // TODO: what about update and create?
  respondForQueryRecord: function (respond, query) {
    return this._super(function (cb) {
      return respond((headers, body) => {
        body.Roles = typeof body.Roles === 'undefined' || body.Roles === null ? [] : body.Roles;
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
            item.Roles = typeof item.Roles === 'undefined' || item.Roles === null ? [] : item.Roles;
            return item;
          })
        );
      });
    }, query);
  },
  serialize: function (snapshot, options) {
    const data = this._super(...arguments);
    
    // In Ember Data 4.12+, we need to access the actual model record
    // to get the Roles before they're serialized away
    let roles = [];
    if (snapshot && snapshot.record && snapshot.record.Roles) {
      roles = snapshot.record.Roles.toArray ? snapshot.record.Roles.toArray() : snapshot.record.Roles;
    } else if (data.Roles) {
      roles = data.Roles;
    }
    
    data.Roles = minimizeModel(roles);
    return data;
  },
});
