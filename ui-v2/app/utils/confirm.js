import { Promise } from 'rsvp';
export default function(message, confirmation = confirm) {
  return Promise.resolve(confirmation(message));
}
