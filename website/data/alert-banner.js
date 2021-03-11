export const ALERT_BANNER_ACTIVE = true

// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  tag: 'New',
  url:
    'https://cloud.hashicorp.com/?utm_source=consul_io&utm_content=alert_banner',
  text: 'HCP Consul is now generally available on AWS',
  linkText: 'Try today',
  // Set the `expirationDate prop with a datetime string (e.g. `2020-01-31T12:00:00-07:00`)
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: '2021-04-14T11:59:00-05:00',
}
