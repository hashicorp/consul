/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { set, computed } from '@ember/object';
import { inject as service } from '@ember/service';

/**
 * Available syntax highlighting languages for the KV editor.
 * Each entry contains:
 * - value: The language identifier used by the HDS components
 * - label: The display label shown in the dropdown
 * - codeBlockLanguage: The fallback language for CodeBlock (which has different supported languages)
 */
const LANGUAGES = [
  { value: 'json', label: 'JSON', codeBlockLanguage: 'json' },
  { value: 'yaml', label: 'YAML', codeBlockLanguage: 'yaml' },
  { value: 'hcl', label: 'HCL', codeBlockLanguage: 'hcl' },
  { value: 'toml', label: 'TOML', codeBlockLanguage: 'hcl' }, // TOML uses HCL as CodeBlock fallback (similar syntax)
];

export default Component.extend({
  tagName: '',
  encoder: service('btoa'),
  json: true,
  language: 'json', // Default syntax highlighting language

  /**
   * Available languages for the syntax highlighting dropdown.
   */
  languages: LANGUAGES,

  ondelete: function () {
    this.onsubmit(...arguments);
  },
  oncancel: function () {
    this.onsubmit(...arguments);
  },
  onsubmit: function () {},

  /**
   * Gets the appropriate CodeBlock language.
   * For TOML, falls back to HCL since CodeBlock doesn't natively support TOML.
   */
  codeBlockLanguage: computed('language', function () {
    const lang = LANGUAGES.find((l) => l.value === this.language);
    return lang ? lang.codeBlockLanguage : 'json';
  }),

  actions: {
    change: function (e, form) {
      const item = form.getData();
      try {
        form.handleEvent(e);
      } catch (err) {
        const target = e.target;
        let parent;
        switch (target.name) {
          case 'value':
            set(item, 'Value', this.encoder.execute(target.value));
            break;
          case 'additional':
            parent = this.parent;
            set(item, 'Key', `${parent !== '/' ? parent : ''}${target.value}`);
            break;
          case 'json':
            // TODO: Potentially save whether json has been clicked to the model,
            // setting set(this, 'json', true) here will force the form to always default to code=on
            // even if the user has selected code=off on another KV
            // ideally we would save the value per KV, but I'd like to not do that on the model
            // a set(this, 'json', valueFromSomeStorageJustForThisKV) would be added here
            set(this, 'json', !this.json);
            break;
          case 'language':
            set(this, 'language', target.value);
            break;
          default:
            throw err;
        }
      }
    },
  },
});
