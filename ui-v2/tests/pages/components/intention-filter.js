import { triggerable } from 'ember-cli-page-object';
import radiogroup from 'consul-ui/tests/lib/page-object/radiogroup';
export default {
  ...radiogroup('currentFilter', ['', 'allow', 'deny']),
  ...{
    scope: '[data-test-intention-filter]',
    search: triggerable('keypress', '[name="s"]'),
  },
};
