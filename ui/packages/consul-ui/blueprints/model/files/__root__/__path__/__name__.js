/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from 'ember-data/model';
import attr from 'ember-data/attr';
//import { nullValue } from 'consul-ui/decorators/replace';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';
export default class <%= classifiedModuleName %>Model extends Model {
  @attr('string') uid;
  @attr('string') ID;
  @attr('string') Datacenter;

  // @attr('string') Namespace; // Does this Model support namespaces?

  // @nullValue([]) @attr({ defaultValue: () => [] }) MaybeNull; // Does a property sometimes return null?

  // @attr('number') SyncTime; // Does this Model support blocking queries?
  // @attr() meta; // {} // Does this Model support blocking queries?
}
