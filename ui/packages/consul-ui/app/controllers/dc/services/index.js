/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import PeeredResourceController from 'consul-ui/controllers/_peered-resource';
import { inject as service } from '@ember/service';

export default class DcServicesController extends PeeredResourceController {
  @service router;

  get shouldShowLinkAlert() {
    return !this.router.currentURL.includes('/link');
  }
}
