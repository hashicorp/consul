import Promise from 'rsvp';
export default function(message)
{
  if(confirm(message)) {
    return Promise.resolve(message);
  } else {
    return Promise.reject(message);
  }
}
