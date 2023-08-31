/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Controller from "@ember/controller";
import { tracked } from "@glimmer/tracking";
import { action } from "@ember/object";

export default class PeersEditExportedController extends Controller {
  queryParams = {
    search: {
      as: "filter",
    },
  };

  @tracked search = "";

  @action updateSearch(value) {
    this.search = value;
  }
}
