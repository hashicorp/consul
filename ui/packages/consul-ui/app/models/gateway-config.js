/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { array } from 'ember-data-model-fragments/attributes';
import { attr } from '@ember-data/model';

export default class GatewayConfig extends Fragment {
  // AssociatedServiceCount is only populated when asking for a list of
  // services
  @attr('number', { defaultValue: () => 0 }) AssociatedServiceCount;
  // Addresses is only populated when asking for a list of services for a
  // specific gateway
  @array('string', { defaultValue: () => [] }) Addresses;
}
