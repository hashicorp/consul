import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

export const schema = {
  Name: {
    required: true,
  },
  HeaderType: {
    allowedValues: ['Exact', 'Prefix', 'Suffix', 'Regex', 'Present'],
  },
};

export default class IntentionPermission extends Fragment {
  @attr('string') Name;

  @attr('string') Exact;
  @attr('string') Prefix;
  @attr('string') Suffix;
  @attr('string') Regex;
  // this is a boolean but we don't want it to automatically be set to false
  @attr() Present;

  @or(...schema.HeaderType.allowedValues) Value;

  @computed(...schema.HeaderType.allowedValues)
  get HeaderType() {
    return schema.HeaderType.allowedValues.find((prop) => typeof this[prop] !== 'undefined');
  }
}
