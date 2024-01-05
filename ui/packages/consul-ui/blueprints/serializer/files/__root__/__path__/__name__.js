/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/<%= dasherizedModuleName  %>';

export default class <%= classifiedModuleName %>Serializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  // respondForQueryRecord(respond, query) {
  //   return super.respondForQueryRecord(
  //     function(cb) {
  //       return respond(
  //         function(headers, body) {

  //           return cb(
  //             headers,
  //             body
  //           );

  //         }
  //       )
  //     },
  //     query
  //   );
  // }
}
