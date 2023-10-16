/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Serializer from './application';
import { get } from '@ember/object';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/token';
import WithPolicies from 'consul-ui/mixins/policy/as-many';
import WithRoles from 'consul-ui/mixins/role/as-many';

export default class TokenSerializer extends Serializer.extend(WithPolicies, WithRoles) {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  serialize(snapshot, options) {
    let data = super.serialize(...arguments);
    // If a token has Rules, use the old API shape notice we use a null check
    // here (not an undefined check) as we are dealing with the serialized
    // model not raw user data
    if (data['Rules'] !== null) {
      data = {
        ID: data.SecretID,
        Name: data.Description,
        Type: data.Type,
        Rules: data.Rules,
      };
    }
    // make sure we never send the SecretID
    // TODO: If we selectively format the request payload in the adapter we
    // won't have to do this here see side note in
    // https://github.com/hashicorp/consul/pull/6285 which will mean most if
    // not all of this method can go
    if (data) {
      delete data['SecretID'];
    }
    return data;
  }

  respondForSelf(respond, query) {
    return this.respondForQueryRecord(respond, query);
  }

  respondForUpdateRecord(respond, serialized, data) {
    return super.respondForUpdateRecord(
      (cb) =>
        respond((headers, body) => {
          // Sometimes we get `Policies: null`, make null equal an empty array
          if (typeof body.Policies === 'undefined' || body.Policies === null) {
            body.Policies = [];
          }
          // Convert an old style update response to a new style
          if (typeof body['ID'] !== 'undefined') {
            const item = this.store.peekAll('token').findBy('SecretID', body['ID']);
            if (item) {
              body['SecretID'] = body['ID'];
              body['AccessorID'] = get(item, 'AccessorID');
            }
          }
          return cb(headers, body);
        }),
      serialized,
      data
    );
  }
}
