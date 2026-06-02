/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import classic from 'ember-classic-decorator';
import BaseAbility from './base';

@classic
export default class NodeAbility extends BaseAbility {
  resource = 'node';
}
