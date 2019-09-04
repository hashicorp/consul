import Serializer from 'ember-data/serializers/rest';

export default Serializer.extend({
  respondForQuery: function(respond, query) {
    return respond((headers, body) => body);
  },
  respondForQueryRecord: function(respond, query) {
    return respond((headers, body) => body);
  },
  respondForFindAll: function(respond, query) {
    return respond((headers, body) => body);
  },
  respondForCreateRecord: function(respond, data) {
    // TODO: Creates may need a primaryKey adding (remove from application)
    return respond((headers, body) => body);
  },
  respondForUpdateRecord: function(respond, data) {
    // TODO: Updates only need the primaryKey/uid returning (remove from application)
    return respond((headers, body) => body);
  },
  respondForDeleteRecord: function(respond, data) {
    // TODO: Deletes only need the primaryKey/uid returning (remove from application)
    return respond((headers, body) => body);
  },
});
