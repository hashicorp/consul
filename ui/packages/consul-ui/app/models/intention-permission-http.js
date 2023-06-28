/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentArray, array } from 'ember-data-model-fragments/attributes';
import { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

export const schema = {
  PathType: {
    allowedValues: ['PathPrefix', 'PathExact', 'PathRegex'],
  },
  Methods: {
    allowedValues: ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTIONS', 'TRACE', 'PATCH'],
  },
};

export default class IntentionPermissionHttp extends Fragment {
  @attr('string') PathExact;
  @attr('string') PathPrefix;
  @attr('string') PathRegex;

  @fragmentArray('intention-permission-http-header') Header;
  @array('string') Methods;

  @or(...schema.PathType.allowedValues) Path;

  @computed(...schema.PathType.allowedValues)
  get PathType() {
    return schema.PathType.allowedValues.find((prop) => typeof this[prop] === 'string');
  }
}
