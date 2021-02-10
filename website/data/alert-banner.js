export const ALERT_BANNER_ACTIVE = true

// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  tag: 'Announcement',
  url:
    'https://cloud.hashicorp.com/?utm_source=consul_io&utm_content=alert_banner',
  text: 'HashiCorp Consul is now generally available on HCP',
  linkText: 'Learn More',
  // Set the `expirationDate prop with a datetime string (e.g. `2020-01-31T12:00:00-07:00`)
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: '2021-02-14T11:59:00-05:00',
}
