import Subnav from '@hashicorp/react-subnav'
import subnavItems from '../../data/subnav'
import { useRouter } from 'next/router'

export default function ConsulSubnav() {
  const router = useRouter()
  return (
    <Subnav
      hideGithubStars={true}
      titleLink={{
        text: 'consul',
        url: '/',
      }}
      ctaLinks={[
        {
          text: 'GitHub',
          url: 'https://www.github.com/hashicorp/consul',
        },

        { text: 'Download', url: '/downloads' },
        {
          text: 'Try Cloud',
          url:
            'https://cloud.hashicorp.com/?utm_source=consul_io&utm_content=top_nav_consul',
        },
      ]}
      currentPath={router.asPath}
      menuItemsAlign="right"
      menuItems={subnavItems}
      constrainWidth
      matchOnBasePath
    />
  )
}
