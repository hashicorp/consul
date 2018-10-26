import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
const DEFAULTS = {
  tabSize: 2,
  lineNumbers: true,
  theme: 'hashi',
  showCursorWhenSelecting: true,
  gutters: ['CodeMirror-lint-markers'],
  lint: true,
};
export default Component.extend({
  settings: service('settings'),
  helper: service('code-mirror'),
  classNames: ['code-editor'],
  syntax: '',
  onchange: function(value) {
    get(this, 'settings').persist({
      'code-editor': value,
    });
    this.setMode(value);
  },
  onkeyup: function() {},
  init: function() {
    this._super(...arguments);
    set(this, 'modes', get(this, 'helper').modes());
  },
  setMode: function(mode) {
    set(this, 'options', {
      ...DEFAULTS,
      mode: mode.mime,
    });
    const editor = get(this, 'editor');
    editor.setOption('mode', mode.mime);
    get(this, 'helper').lint(editor, mode.mode);
    set(this, 'mode', mode);
  },
  didInsertElement: function() {
    set(this, 'editor', get(this, 'helper').getEditor(this.element));
    get(this, 'settings')
      .findBySlug('code-editor')
      .then(mode => {
        const modes = get(this, 'modes');
        const syntax = get(this, 'syntax');
        if (syntax) {
          mode = modes.find(function(item) {
            return item.name.toLowerCase() == syntax.toLowerCase();
          });
        }
        mode = !mode ? modes[0] : mode;
        this.setMode(mode);
      });
  },
  didAppear: function() {
    get(this, 'editor').refresh();
  },
});
