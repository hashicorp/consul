/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/policy';

export default class PolicySerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;
}
