/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { getOwner } from '@ember/application';

export default class TopologyController extends Controller {
  @action
  createIntention(source, destination) {
    const route = getOwner(this).lookup('route:dc.services.show.topology');
    return route.createIntention(source, destination);
  }
}
