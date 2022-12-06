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
  @attr('string') ServerExternalAddresses;
  @nullValue([]) @attr() ServerExternalAddresses;

  // only the side that establishes will hold this property
  @attr('string') PeerID;

  @attr() PeerServerAddresses;

  // StreamStatus
  @nullValue([]) @attr() ImportedServices;
  @nullValue([]) @attr() ExportedServices;
  @attr('date') LastHeartbeat;
  @attr('date') LastReceive;
  @attr('date') LastSend;

  get ImportedServiceCount() {
    return this.ImportedServices.length;
  }

  get ExportedServiceCount() {
    return this.ExportedServices.length;
  }

  // if we receive a PeerID we know that we are dealing with the side that
  // established the peering
  get isReceiver() {
    return this.PeerID;
  }

  get isDialer() {
    return !this.isReceiver;
  }
}
