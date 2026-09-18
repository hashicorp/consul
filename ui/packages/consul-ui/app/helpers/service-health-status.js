/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { statusFromInstances } from 'consul-ui/utils/health-status';

/**
 * Worst health status across a service's instances, for the overall badge next
 * to the service title. A service page loads service *instances* rather than
 * the aggregated service record the services list uses, so the status has to be
 * derived from them here.
 *
 * @param {Array} instances
 * @returns {string} `critical` | `warning` | `passing` | `empty`
 */
export default helper(([instances]) => statusFromInstances(instances));
