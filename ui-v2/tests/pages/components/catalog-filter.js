import { triggerable } from 'ember-cli-page-object';
import radiogroup from 'consul-ui/tests/lib/page-object/radiogroup';
export default {
  ...radiogroup('status', ['', 'passing', 'warning', 'critical']),
  ...{
    scope: '[data-test-catalog-filter]',
    search: triggerable('keypress', '[name="s"]'),
  },
};
