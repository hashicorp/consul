import attr from 'ember-data/attr';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentArray, array } from 'ember-data-model-fragments/attributes';

export const schema = {
  PathType: {
    allowedValues: ['PathPrefix', 'PathExact', 'PathRegex'],
  },
  Methods: {
    allowedValues: ['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTIONS', 'TRACE', 'PATCH'],
  },
};

export default Fragment.extend({
  PathExact: attr('string'),
  PathPrefix: attr('string'),
  PathRegex: attr('string'),

  Header: fragmentArray('intention-permission-http-header'),
  Methods: array('string'),

  Path: or(...schema.PathType.allowedValues),
  PathType: computed(...schema.PathType.allowedValues, function() {
    return schema.PathType.allowedValues.find(prop => typeof this[prop] === 'string');
  }),
});
