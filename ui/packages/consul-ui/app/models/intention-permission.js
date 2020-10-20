import attr from 'ember-data/attr';

import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';

export const schema = {
  Action: {
    defaultValue: 'allow',
    allowedValues: ['allow', 'deny'],
  },
};

export default Fragment.extend({
  Action: attr('string', {
    defaultValue: schema.Action.defaultValue,
  }),
  HTTP: fragment('intention-permission-http'),
});
