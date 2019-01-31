import Component from '@ember/component';
import { subscribe } from 'consul-ui/utils/computed/purify';
const count = function(value) {
  if (Array.isArray(value)) {
    return value.length;
  }
  return value;
};
export default Component.extend({
  tagName: '',
  passingCount: subscribe('passing', count),
  warningCount: subscribe('warning', count),
  criticalCount: subscribe('critical', count),
});
