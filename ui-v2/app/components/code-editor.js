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
  dom: service('dom'),
  helper: service('code-mirror/linter'),
  classNames: ['code-editor'],
  readonly: false,
  syntax: '',
  // TODO: Change this to oninput to be consistent? We'll have to do it throughout the templates
  onkeyup: function() {},
  oninput: function() {},
  init: function() {
    this._super(...arguments);
    set(this, 'modes', get(this, 'helper').modes());
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    const editor = get(this, 'editor');
    if (editor) {
      editor.setOption('readOnly', get(this, 'readonly'));
    }
  },
  setMode: function(mode) {
    set(this, 'options', {
      ...DEFAULTS,
      mode: mode.mime,
      readOnly: get(this, 'readonly'),
    });
    const editor = get(this, 'editor');
    editor.setOption('mode', mode.mime);
    get(this, 'helper').lint(editor, mode.mode);
    set(this, 'mode', mode);
  },
  willDestroyElement: function() {
    this._super(...arguments);
    if (this.observer) {
      this.observer.disconnect();
    }
  },
  didInsertElement: function() {
    this._super(...arguments);
    const $code = get(this, 'dom').element('textarea ~ pre code', get(this, 'element'));
    if ($code.firstChild) {
      this.observer = new MutationObserver(([e]) => {
        this.oninput(set(this, 'value', e.target.wholeText));
      });
      this.observer.observe($code, {
        attributes: false,
        subtree: true,
        childList: false,
        characterData: true,
      });
      set(this, 'value', $code.firstChild.wholeText);
    }
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
  actions: {
    change: function(value) {
      get(this, 'settings').persist({
        'code-editor': value,
      });
      this.setMode(value);
    },
  },
});
