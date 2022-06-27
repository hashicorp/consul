import Model, { attr } from '@ember-data/model';

export default class Peer extends Model {
  @attr('string') Name;
  @attr('string') State;
  @attr('string') CreateIndex;
  @attr('string') ModifyIndex;
  @attr('number') ImportedServiceCount;
  @attr('number') ExportedServiceCount;
}
