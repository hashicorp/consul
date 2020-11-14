import EnterpriseComparison from '../../enterprise-comparison'

export default function ConsulEnterpriseComparison() {
  return (
    <EnterpriseComparison
      title="When to consider Consul Enterprise"
      itemOne={{
        title: 'Technical Complexity',
        label: 'Open Source',
        imageUrl: require('./img/consul-oss.svg?url'),
        description:
          'Consul Open Source enables individuals to discover services and securely manage connections between them across cloud, on-prem, and hybrid environments.',
        link: {
          text: 'View Open Source Features',
          url: 'https://www.hashicorp.com/products/consul/pricing/',
          type: 'outbound',
        },
      }}
      itemTwo={{
        title: 'Organizational Complexity',
        label: 'Enterprise',
        imageUrl: require('./img/consul-enterprise.svg?url'),
        description:
          'Consul Enterprise provides the foundation for organizations to build an enterprise-ready service networking environment for multiple teams by enabling governance capabilities.',
        link: {
          text: 'View Enterprise Features',
          url: 'https://www.hashicorp.com/products/consul/pricing/',
          type: 'outbound',
        },
      }}
      brand="consul"
    />
  )
}
