/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import validations from 'consul-ui/validations/policy';
import builderFactory from 'consul-ui/utils/form/builder';
const builder = builderFactory();
export default function (container, name = 'policy', v = validations, form = builder) {
  return form(name, {
    Datacenters: {
      type: 'array',
    },
  }).setValidators(v);
}
