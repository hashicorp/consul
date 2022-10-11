import Model, { attr } from '@ember-data/model';
import { nullValue } from 'consul-ui/decorators/replace';

export const schema = {
  State: {
    defaultValue: 'PENDING',
    allowedValues: ['PENDING', 'ESTABLISHING', 'ACTIVE', 'FAILING', 'TERMINATED', 'DELETING'],
  },
};
export default class Peer extends Model {
  @attr('string') uri;
  @attr() meta;

  @attr('string') Datacenter;
  @attr('string') Partition;

  @attr('string') Name;
  @attr('string') State;
  @attr('string') ID;
  @nullValue([]) @attr() ImportedServices;
  @nullValue([]) @attr() ExportedServices;
  @attr('date') LastHeartbeat;
  @attr('date') LastReceive;
  @attr('date') LastSend;
  @attr() PeerServerAddresses;

  get ImportedServiceCount() {
    return this.ImportedServices.length;
  }
  get ExportedServiceCount() {
    return this.ExportedServices.length;
  }
}
