import $ from 'jquery';
import { Promise } from 'rsvp';
export default function() {
  return new Promise((resolve, reject) => {
    $.ajax(...arguments)
      .then(function(res) {
        resolve(res);
        return res;
      })
      .fail(function(e) {
        reject(e);
        return e;
      });
  });
}
