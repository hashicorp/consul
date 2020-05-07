import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import FeaturedSlider from '@hashicorp/react-featured-slider'

export default function TempUseCaseChildren() {
  return (
    <>
      <div className="with-border">
        <TextSplitWithImage
          textSplit={{
            heading: 'Multi-region, multi-cloud',
            content:
              'Consul’s flexible architecture allows it to be deployed in any environment, in any region, on any cloud.',
            textSide: 'left',
            links: [
              {
                text: 'Learn More',
                url:
                  'https://learn.hashicorp.com/consul?track=datacenter-deploy#datacenter-deploy',
                type: 'outbound',
              },
            ],
          }}
          image={{
            url:
              'https://www.datocms-assets.com/2885/1588822376-multi-region.png',
            alt: '',
          }}
        />
      </div>

      <div className="with-border">
        <TextSplitWithImage
          textSplit={{
            heading: 'Multi-region, multi-cloud',
            content:
              'Consul’s flexible architecture allows it to be deployed in any environment, in any region, on any cloud.',
            textSide: 'right',
            links: [
              {
                text: 'Learn More',
                url:
                  'https://learn.hashicorp.com/consul?track=datacenter-deploy#datacenter-deploy',
                type: 'outbound',
              },
            ],
          }}
          image={{
            url:
              'https://www.datocms-assets.com/2885/1588822376-multi-region.png',
            alt: '',
          }}
        />
      </div>

      <FeaturedSlider
        heading="Case Study"
        theme="dark"
        brand="consul"
        features={[
          {
            logo: {
              url: require('./img/mercedes-logo.svg?url'),
              alt: 'Mercedes-Benz',
            },
            image: {
              url: require('./img/mercedes-card.jpg?url'),
              alt: 'Mercedes-Benz Case Study',
            },
            heading: 'On the Road Again',
            content:
              'How Mercedes-Benz delivers on service networking to accelerate delivery of its next-gen connected vehicles.',
            link: {
              text: 'Read Case Study',
              url: 'https://www.hashicorp.com/case-studies/mercedes/',
              type: 'outbound',
            },
          },
        ]}
      />
    </>
  )
}
