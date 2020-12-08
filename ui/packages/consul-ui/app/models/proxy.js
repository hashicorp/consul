import Model, { attr } from '@ember-data/model';
import ServiceInstanceModel from './service-instance';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Node,ServiceID';

// TODO: This should be changed to ProxyInstance
export default class Proxy extends ServiceInstanceModel {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') ServiceName;
  @attr('string') ServiceID;
  @attr('string') Node;
  @attr('number') SyncTime;
  @attr() ServiceProxy; // {}
}
