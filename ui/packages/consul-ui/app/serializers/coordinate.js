/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/coordinate';

export default class CoordinateSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;
}
