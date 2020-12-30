export const ALERT_BANNER_ACTIVE = true

// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  tag: 'Webinar',
  url:
    'https://www.hashicorp.com/events/webinars/an-introduction-to-federation-on-hcs',
  text: 'An Introduction to Federation on HCS',
  linkText: 'Register Now',
  // Set the `expirationDate prop with a datetime string (e.g. `2020-01-31T12:00:00-07:00`)
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: '2020-12-17T12:00:00-05:00',
}
