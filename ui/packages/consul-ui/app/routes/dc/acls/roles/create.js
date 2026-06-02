/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from './edit';

export default class CreateRoute extends Route {
  get templateName() {
    return 'dc/acls/roles/edit';
  }
}
