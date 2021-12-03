import Subnav from '@hashicorp/react-subnav'
import { useRouter } from 'next/router'
import s from './style.module.css'

export default function ConsulSubnav({ menuItems }) {
  const router = useRouter()
  return (
    <Subnav
      className={s.subnav}
      hideGithubStars={true}
      titleLink={{
        text: 'HashiCorp Consul',
        url: '/',
      }}
      ctaLinks={[
        {
          text: 'GitHub',
          url: 'https://www.github.com/hashicorp/consul',
        },

        { text: 'Download', url: '/downloads' },
        {
          text: 'Try HCP Consul',
          url:
            'https://cloud.hashicorp.com/?utm_source=consul_io&utm_content=top_nav_consul',
          theme: {
            brand: 'consul',
          },
        },
      ]}
      currentPath={router.asPath}
      menuItemsAlign="right"
      menuItems={menuItems}
      constrainWidth
      matchOnBasePath
    />
  )
}
