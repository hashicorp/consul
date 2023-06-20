/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import config from './config/environment';
import getEnvironment from './utils/get-environment';
export const env = getEnvironment(config, window, document);
