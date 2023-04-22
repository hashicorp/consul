/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/session';

export default class SessionSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForQueryRecord(respond, query) {
    return super.respondForQueryRecord(
      (cb) =>
        respond((headers, body) => {
          if (body.length === 0) {
            const e = new Error();
            e.errors = [
              {
                status: '404',
                title: 'Not found',
              },
            ];
            throw e;
          }
          return cb(headers, body[0]);
        }),
      query
    );
  }
}
