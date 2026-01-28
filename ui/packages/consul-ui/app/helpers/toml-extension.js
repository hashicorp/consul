/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { StreamLanguage } from '@codemirror/language';
import { toml } from '@codemirror/legacy-modes/mode/toml';

/**
 * Helper to provide TOML syntax highlighting extension for CodeMirror 6.
 * Used with the HDS CodeEditor component via @customExtensions.
 *
 * @returns {Array} Array containing the TOML StreamLanguage extension
 */
export function tomlExtension() {
  return [StreamLanguage.define(toml)];
}

export default helper(tomlExtension);
