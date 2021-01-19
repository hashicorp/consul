import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';

export default class GatewayConfig extends Fragment {
  @attr('number', { defaultValue: () => 0 }) AssociatedServiceCount;
}
