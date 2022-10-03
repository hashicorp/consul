import Component from '@ember/component';
import { set } from '@ember/object';
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
  onkeyup: function () {},
  oninput: function () {},
  init: function () {
    this._super(...arguments);
    set(this, 'modes', this.helper.modes());
  },
  didReceiveAttrs: function () {
    this._super(...arguments);
    const editor = this.editor;
    if (editor) {
      editor.setOption('readOnly', this.readonly);
    }
  },
  setMode: function (mode) {
    let options = {
      ...DEFAULTS,
      mode: mode.mime,
      readOnly: this.readonly,
    };
    if (mode.name === 'XML') {
      options.htmlMode = mode.htmlMode;
      options.matchClosing = mode.matchClosing;
      options.alignCDATA = mode.alignCDATA;
    }
    set(this, 'options', options);

    const editor = this.editor;
    editor.setOption('mode', mode.mime);
    this.helper.lint(editor, mode.mode);
    set(this, 'mode', mode);
  },
  willDestroyElement: function () {
    this._super(...arguments);
    if (this.observer) {
      this.observer.disconnect();
    }
  },
  didInsertElement: function () {
    this._super(...arguments);
    const $code = this.dom.element('textarea ~ pre code', this.element);
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
    set(this, 'editor', this.helper.getEditor(this.element));
    this.settings.findBySlug('code-editor').then((mode) => {
      const modes = this.modes;
      const syntax = this.syntax;
      if (syntax) {
        mode = modes.find(function (item) {
          return item.name.toLowerCase() == syntax.toLowerCase();
        });
      }
      mode = !mode ? modes[0] : mode;
      this.setMode(mode);
    });
  },
  didAppear: function () {
    this.editor.refresh();
  },
  actions: {
    change: function (value) {
      this.settings.persist({
        'code-editor': value,
      });
      this.setMode(value);
    },
  },
});
