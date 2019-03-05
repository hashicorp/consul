import { triggerable } from 'ember-cli-page-object';
export default {
  search: triggerable('keypress', '[name="s"]'),
};
