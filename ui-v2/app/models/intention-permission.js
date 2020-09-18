import attr from 'ember-data/attr';

import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  Action: attr('string', { defaultValue: 'allow' }),
  Http: fragment('intention-permission-http'),
});
