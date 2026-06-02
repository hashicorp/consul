/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import config from './config/environment';
import getEnvironment from './utils/get-environment';
export const env = getEnvironment(config, window, document);
