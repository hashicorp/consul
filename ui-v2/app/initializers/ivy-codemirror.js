import { inject as service } from '@ember/service';
import { get } from '@ember/object';
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
];
export function initialize(application) {
  const IvyCodeMirrorComponent = application.resolveRegistration('component:ivy-codemirror');
  const IvyCodeMirrorService = application.resolveRegistration('service:code-mirror');
  // Make sure ivy-codemirror respects/maintains a `name=""` attribute
  IvyCodeMirrorComponent.reopen({
    attributeBindings: ['name'],
  });
  // Add some method to the code-mirror service so I don't have to have 2 services
  // for dealing with codemirror
  IvyCodeMirrorService.reopen({
    dom: service('dom'),
    modes: function() {
      return MODES;
    },
    lint: function() {
      return lint(...arguments);
    },
    getEditor: function(element) {
      return get(this, 'dom').element('textarea + div', element).CodeMirror;
    },
  });
}

export default {
  initialize,
};
