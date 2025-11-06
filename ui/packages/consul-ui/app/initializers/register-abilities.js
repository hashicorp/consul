/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import PeerAbility from '../abilities/peer';
import AclAbility from '../abilities/acl';
import AuthMethodAbility from '../abilities/auth-method';
import IntentionAbility from '../abilities/intention';
import KvAbility from '../abilities/kv';
import LicenseAbility from '../abilities/license';
import NodeAbility from '../abilities/node';
import NspaceAbility from '../abilities/nspace';
import OverviewAbility from '../abilities/overview';
import PartitionAbility from '../abilities/partition';
import PermissionAbility from '../abilities/permission';
import PolicyAbility from '../abilities/policy';
import RoleAbility from '../abilities/role';
import ServerAbility from '../abilities/server';
import ServiceInstanceAbility from '../abilities/service-instance';
import SessionAbility from '../abilities/session';
import TokenAbility from '../abilities/token';
import UpstreamAbility from '../abilities/upstream';
import ZerviceAbility from '../abilities/zervice';
import ZoneAbility from '../abilities/zone';

export function initialize(app) {
  app.register('ability:peer', PeerAbility);
  app.register('ability:acl', AclAbility);
  app.register('ability:auth-method', AuthMethodAbility);
  app.register('ability:intention', IntentionAbility);
  app.register('ability:kv', KvAbility);
  app.register('ability:license', LicenseAbility);
  app.register('ability:node', NodeAbility);
  app.register('ability:nspace', NspaceAbility);
  app.register('ability:overview', OverviewAbility);
  app.register('ability:partition', PartitionAbility);
  app.register('ability:permission', PermissionAbility);
  app.register('ability:policy', PolicyAbility);
  app.register('ability:role', RoleAbility);
  app.register('ability:server', ServerAbility);
  app.register('ability:service-instance', ServiceInstanceAbility);
  app.register('ability:session', SessionAbility);
  app.register('ability:token', TokenAbility);
  app.register('ability:upstream', UpstreamAbility);
  app.register('ability:zervice', ZerviceAbility);
  app.register('ability:zone', ZoneAbility);
}

export default {
  name: 'register-abilities',
  initialize
};