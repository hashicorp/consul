export default [
  { text: 'Overview', url: '/', type: 'inbound' },
  {
    text: 'Use Cases',
    submenu: [
      { text: 'Service Discovery', url: '/discovery' },
      { text: 'Service Mesh', url: '/mesh' },
    ],
  },
  {
    text: 'Enterprise',
    url:
      'https://www.hashicorp.com/products/consul/?utm_source=oss&utm_medium=header-nav&utm_campaign=consul',
    type: 'outbound',
  },
  'divider',
  {
    text: 'Learn',
    url: 'https://learn.hashicorp.com/consul',
    type: 'outbound',
  },
  {
    text: 'Docs',
    url: '/docs',
    type: 'inbound',
  },
  {
    text: 'API',
    url: '/api-docs',
    type: 'inbound',
  },
  {
    text: 'Community',
    url: '/community',
    type: 'inbound',
  },
]
