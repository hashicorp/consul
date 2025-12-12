/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class DcServicesShowIndexController extends Controller {
  @service router;

  @action
  forward(tabs = {}) {
    if (tabs.topology) return this.router.replaceWith('dc.services.show.topology');
    if (tabs.upstreams) return this.router.replaceWith('dc.services.show.upstreams');
    if (tabs.services) return this.router.replaceWith('dc.services.show.services');
    return this.router.replaceWith('dc.services.show.instances');
  }
}
