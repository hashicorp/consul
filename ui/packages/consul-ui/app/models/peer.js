import Model, { attr } from '@ember-data/model';

export const schema = {
  State: {
    defaultValue: 'PENDING',
    allowedValues: [
      'PENDING',
      'ESTABLISHING',
      'ACTIVE',
      'FAILING',
      'TERMINATED',
      'DELETING'
    ],
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
  @attr('number') ImportedServiceCount;
  @attr('number') ExportedServiceCount;
  @attr() PeerServerAddresses;
}
