import attr from 'ember-data/attr';

import Fragment from 'ember-data-model-fragments/fragment';

export default Fragment.extend({
  Name: attr('string'),

  Exact: attr('string'),
  Prefix: attr('string'),
  Suffix: attr('string'),
  Regex: attr('string'),
  Present: attr('boolean'),
  Invert: attr('boolean'),
});
