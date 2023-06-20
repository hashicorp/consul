/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { run } from '@ember/runloop';

export default function destroyApp(application) {
  run(application, 'destroy');
  if (window.server) {
    window.server.shutdown();
  }
}
