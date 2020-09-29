import attr from 'ember-data/attr';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentArray, array } from 'ember-data-model-fragments/attributes';

const pathProps = ['PathPrefix', 'PathExact', 'PathRegex'];
export default Fragment.extend({
  PathExact: attr('string'),
  PathPrefix: attr('string'),
  PathRegex: attr('string'),

  Header: fragmentArray('intention-permission-http-header'),
  Methods: array('string'),

  Path: or(...pathProps),
  PathType: computed(...pathProps, function() {
    return pathProps.find(prop => typeof this[prop] === 'string');
  }),
});
