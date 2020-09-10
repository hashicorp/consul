import Subnav from '@hashicorp/react-subnav'
import subnavItems from '../../data/subnav'
import { useRouter } from 'next/router'

export default function ConsulSubnav() {
  const router = useRouter()
  return (
    <Subnav
      titleLink={{
        text: 'consul',
        url: '/',
      }}
      ctaLinks={[
        { text: 'GitHub', url: 'https://www.github.com/hashicorp/consul' },
        { text: 'Download', url: '/downloads' },
      ]}
      currentPath={router.pathname}
      menuItemsAlign="right"
      menuItems={subnavItems}
      constrainWidth
    />
  )
}
