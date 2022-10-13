import Controller from '@ember/controller';
import { action } from '@ember/object';

export default class HealthChecksController extends Controller {
  @action
  syntheticNodeSearchPropertyFilter(item, searchProperty) {
    return !(item.Node.Meta?.['synthetic-node'] && searchProperty === 'Node');
  }

  @action
  syntheticNodeHealthCheckFilter(item, healthcheck, index, list) {
    console.log('List to be filtered: ', list);
    console.log(healthcheck.Kind);
    return !(item.Node.Meta?.['synthetic-node'] && healthcheck?.Kind === 'node');
  }
}
