import { clickable, collection } from 'ember-cli-page-object';
export default {
  scope: '[data-popover-select]',
  selected: clickable('button'),
  options: collection('li[role="none"]', {
    button: clickable('button'),
  }),
};
