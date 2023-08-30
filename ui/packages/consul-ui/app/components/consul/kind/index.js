/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { titleize } from 'ember-cli-string-helpers/helpers/titleize';
import { humanize } from 'ember-cli-string-helpers/helpers/humanize';

const normalizedGatewayLabels = {
  'api-gateway': 'API Gateway',
  'mesh-gateway': 'Mesh Gateway',
  'ingress-gateway': 'Ingress Gateway',
  'terminating-gateway': 'Terminating Gateway',
};

export default Component.extend({
  tagName: '',
  Name: computed('item.Kind', function () {
    const name = normalizedGatewayLabels[this.item.Kind];
    return name ? name : titleize(humanize(this.item.Kind));
  }),
});
