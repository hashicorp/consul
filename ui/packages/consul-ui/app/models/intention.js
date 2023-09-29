import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import { fragmentArray } from 'ember-data-model-fragments/attributes';
import replace, { nullValue } from 'consul-ui/decorators/replace';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class Intention extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  @attr('string') Description;

  @replace('', undefined) @attr('string') SourcePeer;
  @attr('string', { defaultValue: () => '*' }) SourceName;
  @attr('string', { defaultValue: () => '*' }) DestinationName;
  @attr('string', { defaultValue: () => 'default' }) SourceNS;
  @attr('string', { defaultValue: () => 'default' }) DestinationNS;
  @attr('string', { defaultValue: () => 'default' }) SourcePartition;
  @attr('string', { defaultValue: () => 'default' }) DestinationPartition;

  @attr('number') Precedence;
  @attr('string', { defaultValue: () => 'consul' }) SourceType;
  @nullValue(undefined) @attr('string') Action;
  @attr('string') LegacyID;
  @attr('boolean', { defaultValue: () => true }) Legacy;
  @attr('number') SyncTime;
  @attr('date') CreatedAt;
  @attr('date') UpdatedAt;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr() Meta; // {}
  @attr({ defaultValue: () => [] }) Resources; // []
  @fragmentArray('intention-permission') Permissions;

  @computed('Meta')
  get IsManagedByCRD() {
    const meta = Object.entries(this.Meta || {}).find(
      ([key, value]) => key === 'external-source' && value === 'kubernetes'
    );
    return typeof meta !== 'undefined';
  }
}
