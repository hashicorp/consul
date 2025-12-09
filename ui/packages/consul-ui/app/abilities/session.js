/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import BaseAbility from './base';
import classic from 'ember-classic-decorator';

@classic
export default class SessionAbility extends BaseAbility {
  resource = 'session';
}
