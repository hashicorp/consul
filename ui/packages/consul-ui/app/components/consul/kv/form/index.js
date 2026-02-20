/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { set, computed } from '@ember/object';
import { inject as service } from '@ember/service';

export const LANGUAGES = [
  { label: 'JSON', value: 'json' },
  { label: 'YAML', value: 'yaml' },
  { label: 'HCL', value: 'hcl' },
  { label: 'TOML', value: 'toml' },
  { label: 'Ruby', value: 'ruby' },
  { label: 'Shell', value: 'shell' },
];

export default Component.extend({
  tagName: '',
  encoder: service('btoa'),
  json: true,
  language: 'json',
  languages: LANGUAGES,
  /**
   * Map the selected language value to the appropriate CodeBlock language.
   * CodeBlock supports: bash, go, hcl, json, log, ruby, shell-session, yaml.
   * For languages not natively supported (e.g. toml), fall back to 'bash'
   * for basic syntax highlighting.
   */
  codeBlockLanguage: computed('language', function () {
    const codeBlockSupported = ['json', 'yaml', 'hcl', 'ruby'];
    const lang = this.language;
    if (codeBlockSupported.includes(lang)) {
      return lang;
    }
    if (lang === 'shell') {
      return 'bash';
    }
    // For TOML and other unsupported languages, use 'bash' as a reasonable fallback
    return 'bash';
  }),
  ondelete: function () {
    this.onsubmit(...arguments);
  },
  oncancel: function () {
    this.onsubmit(...arguments);
  },
  onsubmit: function () {},
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
