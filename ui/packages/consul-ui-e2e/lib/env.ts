/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import fs from 'node:fs';

/** Load `.env` from the working directory, if present. Existing env vars win. */
export function loadDotEnv(file = '.env'): void {
  if (fs.existsSync(file)) process.loadEnvFile(file);
}
