import { helper } from '@ember/component/helper';
// don't worry too much about this as it will
// cease to be a helper anyway
import Ember from 'ember';
export function panelBar(params/*, hash*/) {
  const status = params[0];
  return new Ember.Handlebars.SafeString('<div class="panel-bar ' + (status == "passing" ? "bg-green" : "bg-orange") + '"></div>');
}
export default helper(panelBar);
