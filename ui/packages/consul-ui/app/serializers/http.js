/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Serializer from '@ember-data/serializer/rest';

export default class HttpSerializer extends Serializer {
  transformBelongsToResponse(store, relationship, parent, item) {
    return item;
  }

  transformHasManyResponse(store, relationship, parent, item) {
    return item;
  }

  respondForQuery(respond, query) {
    return respond((headers, body) => body);
  }

  respondForQueryRecord(respond, query) {
    return respond((headers, body) => body);
  }

  respondForFindAll(respond, query) {
    return respond((headers, body) => body);
  }

  respondForCreateRecord(respond, data) {
    // TODO: Creates may need a primaryKey adding (remove from application)
    return respond((headers, body) => body);
  }

  respondForUpdateRecord(respond, data) {
    // TODO: Updates only need the primaryKey/uid returning (remove from
    // application)
    return respond((headers, body) => body);
  }

  respondForDeleteRecord(respond, data) {
    // TODO: Deletes only need the primaryKey/uid returning (remove from application)
    return respond((headers, body) => body);
  }
}
