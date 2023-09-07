/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { run } from '@ember/runloop';

export default function destroyApp(application) {
  run(application, 'destroy');
  if (window.server) {
    window.server.shutdown();
  }
}
