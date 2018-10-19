import Component from '@ember/component';
import qsaFactory from 'consul-ui/utils/dom/qsa-factory';
const $$ = qsaFactory();
export default Component.extend({
  mode: 'application/json',
  classNames: ['code-editor'],
  onkeyup: function() {},
  didAppear: function() {
    const $editor = [...$$('textarea + div', this.element)][0];
    $editor.CodeMirror.refresh();
  },
});
