/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/role';
import WithPolicies from 'consul-ui/mixins/policy/as-many';

export default class RoleSerializer extends Serializer.extend(WithPolicies) {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;
}
