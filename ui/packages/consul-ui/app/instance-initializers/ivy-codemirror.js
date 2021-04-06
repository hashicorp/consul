/* globals CodeMirror */
export function initialize(application) {
  const appName = application.application.name;
  const doc = application.lookup('service:-document');
  // pick codemirror syntax highlighting paths out of index.html
  const fs = JSON.parse(doc.querySelector(`[data-${appName}-fs]`).textContent);
  // configure syntax highlighting for CodeMirror
  CodeMirror.modeURL = {
    replace: function(n, mode) {
      switch (mode) {
        case 'javascript':
          return fs['codemirror/mode/javascript/javascript.js'];
        case 'ruby':
          return fs['codemirror/mode/ruby/ruby.js'];
        case 'yaml':
          return fs['codemirror/mode/yaml/yaml.js'];
      }
    },
  };

  const IvyCodeMirrorComponent = application.resolveRegistration('component:ivy-codemirror');
  // Make sure ivy-codemirror respects/maintains a `name=""` attribute
  IvyCodeMirrorComponent.reopen({
    attributeBindings: ['name'],
  });
}

export default {
  initialize,
};
