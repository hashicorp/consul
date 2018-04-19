import { triggerable } from 'ember-cli-page-object';
import radiogroup from 'consul-ui/tests/lib/page-object/radiogroup';
export default {
  ...radiogroup('type', ['', 'management', 'client']),
  ...{
    scope: '[data-test-acl-filter]',
    search: triggerable('keypress', '[name="s"]'),
  },
};
