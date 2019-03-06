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
    return respond((headers, body) => body);
  },
  respondForUpdateRecord: function(respond, data) {
    return respond((headers, body) => body);
  },
  respondForDeleteRecord: function(respond, data) {
    return respond((headers, body) => body);
  },
});
