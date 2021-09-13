import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';
export const PARTITION_KEY = 'Partition';

export default class PartitionModel extends Model {
  @attr('string') uid;
  @attr('string') Name;
  @attr('string') Description;
  @attr('string') Datacenter;

  // FIXME: Double check entire hierarchy again
  // @attr('string') Namespace; // Does this Model support namespaces?

  @attr('number') SyncTime;
  @attr() meta;
}
