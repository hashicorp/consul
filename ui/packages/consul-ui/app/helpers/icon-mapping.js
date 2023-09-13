/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';

const ICON_MAPPING = {
  kubernetes: 'kubernetes-color',
  terraform: 'terraform-color',
  nomad: 'nomad-color',
  consul: 'consul-color',
  'consul-api-gateway': 'consul-color',
  vault: 'vault',
  aws: 'aws-color',
  'aws-iam': 'aws-color',
  lambda: 'aws-lambda-color',
};

/**
 * Takes a icon name, usually an external-source/auth-method-type, and maps it to a flight-icon name or returns undefined
 * if the icon is not currently mapped to a flight-icon name. This is particularly useful when dealing with converting icons to
 * use the `<FlightIcon>` component directly instead of our own css. If the icon is not available with `<FlightIcon>` you can leave
 * it out of the mapping and handle it in the undefined case.
 *
 * @param {string} icon
 * @returns {string|undefined}
 */
function iconMapping([icon]) {
  return ICON_MAPPING[icon];
}

export default helper(iconMapping);
