/* globals CodeMirror */
export function initialize(application) {
  const appName = application.application.name;
  const doc = application.lookup('service:-document');
  // pick codemirror syntax highlighting paths out of index.html
  const fs = new Map(
    Object.entries(JSON.parse(doc.querySelector(`[data-${appName}-fs]`).textContent))
  );
  // configure syntax highlighting for CodeMirror
  CodeMirror.modeURL = {
    replace: function(n, mode) {
      switch (mode.trim()) {
        case 'javascript':
          return fs.get(['codemirror', 'mode', 'javascript', 'javascript.js'].join('/'));
        case 'ruby':
          return fs.get(['codemirror', 'mode', 'ruby', 'ruby.js'].join('/'));
        case 'yaml':
          return fs.get(['codemirror', 'mode', 'yaml', 'yaml.js'].join('/'));
        case 'xml':
          return fs.get(['codemirror', 'mode', 'xml', 'xml.js'].join('/'));
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
