import { computed } from '@ember/object';
import { or } from '@ember/object/computed';
import attr from 'ember-data/attr';

import Fragment from 'ember-data-model-fragments/fragment';

export const schema = {
  Name: {
    required: true,
  },
  HeaderType: {
    allowedValues: ['Exact', 'Prefix', 'Suffix', 'Regex', 'Present'],
  },
};

export default Fragment.extend({
  Name: attr('string'),

  Exact: attr('string'),
  Prefix: attr('string'),
  Suffix: attr('string'),
  Regex: attr('string'),
  Present: attr(), // this is a boolean but we don't want it to automatically be set to false

  Value: or(...schema.HeaderType.allowedValues),
  HeaderType: computed(...schema.HeaderType.allowedValues, function() {
    return schema.HeaderType.allowedValues.find(prop => typeof this[prop] !== 'undefined');
  }),
});
