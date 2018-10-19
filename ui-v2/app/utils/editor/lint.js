/*global CodeMirror*/

// CodeMirror doesn't seem to have anyway to hook into whether a mode
// has already loaded, or when a mode has finished loading
// follow more or less what CodeMirror does but doesn't expose
// see codemirror/addon/mode/loadmode.js

export const createLoader = function(
  $$ = document.getElementsByTagName.bind(document),
  CM = CodeMirror
) {
  CM.registerHelper('lint', 'ruby', function(text) {
    return [];
  });
  return function(editor, mode, cb) {
    let scripts = [...$$('script')];
    const loaded = scripts.find(function(item) {
      return item.src.indexOf(`/codemirror/mode/${mode}/${mode}.js`) !== -1;
    });
    CM.autoLoadMode(editor, mode);
    if (loaded) {
      cb();
    } else {
      scripts = [...$$('script')];
      CM.on(scripts[0], 'load', function() {
        cb();
      });
    }
  };
};
const load = createLoader();
export default function(editor, mode) {
  load(editor, mode, function() {
    if (editor.getValue().trim().length) {
      editor.performLint();
    }
  });
}
