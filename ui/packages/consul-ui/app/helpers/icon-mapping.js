/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

// External sources render as monochrome glyphs so the badges read as a
// consistent set rather than a row of competing brand colours; they inherit
// the badge's foreground colour via `currentColor`.
const ICON_MAPPING = {
  kubernetes: 'kubernetes',
  terraform: 'terraform',
  nomad: 'nomad',
  consul: 'consul',
  'consul-api-gateway': 'consul',
  vault: 'vault',
  aws: 'aws',
  'aws-iam': 'aws',
  lambda: 'aws-lambda',
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
