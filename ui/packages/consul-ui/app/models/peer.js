import Model, { attr } from '@ember-data/model';

export default class Peer extends Model {
  @attr('string') name;
  @attr('string') state;
  @attr('string') createIndex;
  @attr('string') modifyIndex;
}
