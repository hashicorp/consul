/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint no-console: ["error", { allow: ["debug"] }] */
import { runInDebug } from '@ember/debug';

// if we can't find the message, take the last part of the identifier and
// ucfirst it so it looks human
export default function missingMessage(key, locales) {
  runInDebug((_) => console.debug(`Translation key not found: ${key}`));
  const last = key.split('.').pop().split('-').join(' ');
  return `${last.substr(0, 1).toUpperCase()}${last.substr(1)}`;
}
