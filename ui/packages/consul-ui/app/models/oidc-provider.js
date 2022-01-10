import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';

export default class OidcProvider extends Model {
  @attr('string') uid;
  @attr('string') Name;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string') Kind;
  @attr('string') AuthURL;
  @attr('string') DisplayName;
  @attr() meta; // {}
}
