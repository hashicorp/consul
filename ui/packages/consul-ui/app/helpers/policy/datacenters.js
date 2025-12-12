/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

/**
 * Datacenters can be an array of datacenters.
 * Anything that isn't an array means 'All', even an empty array.
 */
export function datacenters(params, hash = {}) {
  const datacenters = params[0]?.Datacenters;
  if (!Array.isArray(datacenters) || datacenters.length === 0) {
    return [hash['global'] || 'All'];
  }
  return datacenters;
}

export default helper(datacenters);
