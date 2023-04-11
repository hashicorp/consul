import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';
import { attr } from '@ember-data/model';

export const schema = {
  Action: {
    defaultValue: 'allow',
    allowedValues: ['allow', 'deny'],
  },
};

export default class IntentionPermission extends Fragment {
  @attr('string', { defaultValue: () => schema.Action.defaultValue }) Action;
  @fragment('intention-permission-http') HTTP;
}
