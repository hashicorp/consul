import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/<%= dasherizedModuleName  %>';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  // respondForQueryRecord: function(respond, query) {
  //   return this._super(
  //     function(cb) {
  //       return respond(
  //         function(headers, body) {

  //           return cb(
  //             headers,
  //             body
  //           );

  //         }
  //       )
  //     },
  //     query
  //   );
  // },
});
