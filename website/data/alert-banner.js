export const ALERT_BANNER_ACTIVE = true

// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  tag: 'Announcing',
  url: 'https://www.hashicorp.com/blog/announcing-hashicorp-consul-1-9',
  text: 'HashiCorp Consul 1.9 now available in beta.',
  linkText: 'Learn more',
  // Set the `expirationDate prop with a datetime string (e.g. `2020-01-31T12:00:00-07:00`)
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: null,
}
