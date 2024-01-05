/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';
import lint from 'consul-ui/utils/editor/lint';
const MODES = [
  {
    name: 'JSON',
    mime: 'application/json',
    mode: 'javascript',
    ext: ['json', 'map'],
    alias: ['json5'],
  },
  {
    name: 'HCL',
    mime: 'text/x-ruby',
    mode: 'ruby',
    ext: ['rb'],
    alias: ['jruby', 'macruby', 'rake', 'rb', 'rbx'],
  },
  { name: 'YAML', mime: 'text/x-yaml', mode: 'yaml', ext: ['yaml', 'yml'], alias: ['yml'] },
  {
    name: 'XML',
    mime: 'application/xml',
    mode: 'xml',
    htmlMode: false,
    matchClosing: true,
    alignCDATA: false,
    ext: ['xml'],
    alias: ['xml'],
  },
];

export default class LinterService extends Service {
  @service('dom')
  dom;

  modes() {
    return MODES;
  }

  lint() {
    return lint(...arguments);
  }

  getEditor(element) {
    return this.dom.element('textarea + div', element).CodeMirror;
  }
}
