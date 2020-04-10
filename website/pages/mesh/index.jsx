import CallToAction from '@hashicorp/react-call-to-action'
import BeforeAfterDiagram from '../../components/before-after'

export default function ServiceMesh() {
  return (
    <>
      <div className="consul-connect">
        <CallToAction
          heading="Service Mesh made easy"
          content="Service discovery, identity-based authorization, and L7 traffic management abstracted from application code with proxies in the service mesh pattern"
          brand="consul"
          links={[
            { text: 'Download', url: '/downloads' },
            {
              text: 'Explore Docs',
              url:
                'https://learn.hashicorp.com/consul/getting-started/services',
            },
          ]}
        />
        <section
          id="static-dynamic"
          className="g-section-block layout-vertical theme-white-background-black-text small-padding"
        >
          <div className="g-grid-container">
            <BeforeAfterDiagram
              beforeHeading="The Challenge"
              beforeSubTitle="Network appliances, like load balancers or firewalls with manual processes, don't scale in dynamic settings to support modern applications."
              beforeImage="/img/consul-connect/svgs/segmentation-challenge.svg"
              beforeDescription="East-west firewalls use IP-based rules to secure ingress and egress traffic. But in a dynamic world where services move across machines and machines are frequently created and destroyed, this perimeter-based approach is difficult to scale as it results in complex network topologies and a sprawl of short-lived firewall rules and proxy configuration."
              afterHeading="The Solution"
              afterSubTitle="Service mesh as an automated and distributed approach to networking and security that can operate across platforms and private and public cloud"
              afterImage="/img/consul-connect/svgs/segmentation-solution.svg"
              afterDescription="Service mesh is a new approach to secure the service itself rather than relying on the network. Consul uses centrally managed service policies and configuration to enable dynamic routing and security based on service identity. These policies scale across datacenters and large fleets without IP-based rules or networking middleware."
            />
          </div>
        </section>

        <section class="g-section g-cta-section large-padding">
          <div>
            <h2 class="g-type-display-2">Ready to get started?</h2>
            <a href="/downloads.html" class="button download white">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="20"
                height="22"
                viewBox="0 0 20 22"
              >
                <path d="M9.292 15.706a1 1 0 0 0 1.416 0l3.999-3.999a1 1 0 1 0-1.414-1.414L11 12.586V1a1 1 0 1 0-2 0v11.586l-2.293-2.293a1 1 0 1 0-1.414 1.414l3.999 3.999zM20 16v3c0 1.654-1.346 3-3 3H3c-1.654 0-3-1.346-3-3v-3a1 1 0 1 1 2 0v3c0 .551.448 1 1 1h14c.552 0 1-.449 1-1v-3a1 1 0 1 1 2 0z" />
              </svg>
              Download
            </a>
            <a
              href="https://learn.hashicorp.com/consul/getting-started/services"
              class="button secondary white"
            >
              Explore docs
            </a>
          </div>
        </section>
      </div>
    </>
  )
}
