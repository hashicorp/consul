import Model, { attr } from '@ember-data/model';

export default class Peer extends Model {
  @attr('string') uri;

  @attr('string') Datacenter;
  @attr('string') Partition;

  @attr('string') Name;
  @attr('string') State;
  @attr('number') ImportedServiceCount;
  @attr('number') ExportedServiceCount;
  @attr() PeerServerAddresses;
}
