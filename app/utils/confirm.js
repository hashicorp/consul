import Promise from 'rsvp';
export default function(message) {
  return Promise.resolve(confirm(message));
}
