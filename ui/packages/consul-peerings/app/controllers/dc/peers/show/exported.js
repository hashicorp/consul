/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from "@ember/controller";
import { tracked } from "@glimmer/tracking";
import { action } from "@ember/object";

export default class PeersEditExportedController extends Controller {
  queryParams = ['search'];

  @tracked search = "";

  @action updateSearch(value) {
    this.search = value;
  }
}
