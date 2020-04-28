import CallToAction from '@hashicorp/react-call-to-action'
import CodeBlock from '@hashicorp/react-code-block'
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

        <section class="g-section border-top large-padding">
          <div class="g-container">
            <div class="intro">
              <h2 class="g-type-display-2">Features</h2>
            </div>
            <div class="g-text-asset reverse">
              <div>
                <div>
                  <h3 class="g-type-display-3">Layer 7 Traffic Management</h3>
                  <p class="g-type-body">
                    Service-to-service communication policy at Layer 7 can be
                    managed centrally, enabling advanced traffic management
                    patterns such as service failover, path-based routing, and
                    traffic shifting that can be applied across public and
                    private clouds, platforms, and networks.
                  </p>
                  <p>
                    <a
                      class="learn-more g-type-buttons-and-standalone-links"
                      href="/docs/connect/l7-traffic-management.html"
                    >
                      Learn more
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="6"
                        height="10"
                        viewBox="0 0 6 10"
                      >
                        <g
                          fill="none"
                          fillRule="evenodd"
                          transform="translate(-6 -3)"
                        >
                          <mask id="a" fill="#fff">
                            <path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z" />
                          </mask>
                          <g fill="#1563FF" mask="url(#a)">
                            <path d="M0 0h16v16H0z" />
                          </g>
                        </g>
                      </svg>
                    </a>
                  </p>
                </div>
              </div>
              <div class="code-sample">
                <div>
                  <span></span>
                  <CodeBlock
                    prefix="terminal"
                    code={`
Kind = "service-splitter"
Name = "billing-api"

Splits = [
    {
        Weight        = 10
        ServiceSubset = "v2"
    },
    {
        Weight        = 90
        ServiceSubset = "v1"
    },
]
          `}
                  />
                </div>
              </div>
            </div>
          </div>
        </section>

        <section class="g-section border-top large-padding">
          <div class="g-container">
            <div class="g-text-asset large">
              <div>
                <div>
                  <h3 class="g-type-display-3">Layer 7 Observability</h3>
                  <p class="g-type-body">
                    Centrally managed service observability at Layer 7 including
                    detailed metrics on all service-to-service communication
                    such as connections, bytes transferred, retries, timeouts,
                    open circuits, and request rates, response codes.
                  </p>
                  <p>
                    <a
                      class="learn-more g-type-buttons-and-standalone-links"
                      href="/docs/connect/observability.html"
                    >
                      Learn more
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="6"
                        height="10"
                        viewBox="0 0 6 10"
                      >
                        <g
                          fill="none"
                          fillRule="evenodd"
                          transform="translate(-6 -3)"
                        >
                          <mask id="a" fill="#fff">
                            <path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z" />
                          </mask>
                          <g fill="#1563FF" mask="url(#a)">
                            <path d="M0 0h16v16H0z" />
                          </g>
                        </g>
                      </svg>
                    </a>
                  </p>
                </div>
              </div>
              <div>
                <picture>
                  <source
                    type="image/png"
                    srcSet="/img/consul-connect/mesh-observability/metrics_300.png 300w, /img/consul-connect/mesh-observability/metrics_976.png 976w, /img/consul-connect/mesh-observability/metrics_1200.png 1200w"
                  />
                  <img
                    src="/img/consul-connect/mesh-observability/metrics_1200.png"
                    alt="Metrics dashboard"
                  />
                </picture>
              </div>
            </div>
          </div>
        </section>

        <section class="g-section border-top large-padding">
          <div class="g-container">
            <div class="g-text-asset reverse">
              <div>
                <div>
                  <h3 class="g-type-display-3">
                    Secure services across any runtime platform
                  </h3>
                  <p class="g-type-body">
                    Secure communication between legacy and modern workloads.
                    Sidecar proxies allow applications to be integrated without
                    code changes and Layer 4 support provides nearly universal
                    protocol compatibility.
                  </p>
                  <p>
                    <a
                      class="learn-more g-type-buttons-and-standalone-links"
                      href="/docs/connect/proxies.html"
                    >
                      Learn more
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="6"
                        height="10"
                        viewBox="0 0 6 10"
                      >
                        <g
                          fill="none"
                          fillRule="evenodd"
                          transform="translate(-6 -3)"
                        >
                          <mask id="a" fill="#fff">
                            <path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z" />
                          </mask>
                          <g fill="#1563FF" mask="url(#a)">
                            <path d="M0 0h16v16H0z" />
                          </g>
                        </g>
                      </svg>
                    </a>
                  </p>
                </div>
              </div>
              <div>
                <picture>
                  <source
                    type="image/webp"
                    srcSet="
                      /img/consul-connect/grid_3/grid_3_300.webp 300w,
                      /img/consul-connect/grid_3/grid_3_976.webp 976w,
                      /img/consul-connect/grid_3/grid_3_1256.webp 1256w"
                  />
                  <source
                    type="image/png"
                    srcSet="
                      /img/consul-connect/grid_3/grid_3_300.png 300w,
                      /img/consul-connect/grid_3/grid_3_976.png 976w,
                      /img/consul-connect/grid_3/grid_3_1256.png 1256w"
                  />
                  <img
                    src="/img/consul-connect/grid_3/grid_3_1256.png"
                    alt="Secure services across any runtime platform"
                  />
                </picture>
              </div>
            </div>
          </div>
        </section>

        <section class="g-section border-top large-padding">
          <div class="g-container">
            <div class="g-text-asset">
              <div>
                <div>
                  <h3 class="g-type-display-3">
                    Certificate-Based Service Identity
                  </h3>
                  <p class="g-type-body">
                    TLS certificates are used to identify services and secure
                    communications. Certificates use the SPIFFE format for
                    interoperability with other platforms. Consul can be a
                    certificate authority to simplify deployment, or integrate
                    with external signing authorities like Vault.
                  </p>
                  <p>
                    <a
                      class="learn-more g-type-buttons-and-standalone-links"
                      href="/docs/connect/ca.html"
                    >
                      Learn more
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="6"
                        height="10"
                        viewBox="0 0 6 10"
                      >
                        <g
                          fill="none"
                          fillRule="evenodd"
                          transform="translate(-6 -3)"
                        >
                          <mask id="a" fill="#fff">
                            <path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z" />
                          </mask>
                          <g fill="#1563FF" mask="url(#a)">
                            <path d="M0 0h16v16H0z" />
                          </g>
                        </g>
                      </svg>
                    </a>
                  </p>
                </div>
              </div>
              <div class="logos">
                <div>
                  <img src="/img/consul-connect/logos/vault.png" alt="Vault" />
                  <img
                    src="/img/consul-connect/logos/spiffe.png"
                    alt="Spiffe"
                  />
                </div>
              </div>
            </div>
          </div>
        </section>

        <section class="g-section border-top large-padding">
          <div class="g-container">
            <div class="g-text-asset reverse">
              <div>
                <div>
                  <h3 class="g-type-display-3">Encrypted communication</h3>
                  <p class="g-type-body">
                    All traffic between services is encrypted and authenticated
                    with mutual TLS. Using TLS provides a strong guarantee of
                    the identity of services communicating, and ensures all data
                    in transit is encrypted.
                  </p>
                  <p>
                    <a
                      class="learn-more g-type-buttons-and-standalone-links"
                      href="/docs/connect/security.html"
                    >
                      Learn more
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="6"
                        height="10"
                        viewBox="0 0 6 10"
                      >
                        <g
                          fill="none"
                          fillRule="evenodd"
                          transform="translate(-6 -3)"
                        >
                          <mask id="a" fill="#fff">
                            <path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z" />
                          </mask>
                          <g fill="#1563FF" mask="url(#a)">
                            <path d="M0 0h16v16H0z" />
                          </g>
                        </g>
                      </svg>
                    </a>
                  </p>
                </div>
              </div>
              <div class="code-sample">
                <div>
                  <span></span>
                  <CodeBlock
                    prefix="terminal"
                    code={`
$ consul connect proxy -service web \\
    -service-addr 127.0.0.1:8000
    -listen 10.0.1.109:7200
==> Consul Connect proxy starting...
    Configuration mode: Flags
                Service: web
        Public listener: 10.0.1.109:7200 => 127.0.0.1:8000
...
$ tshark -V \\
        -Y "ssl.handshake.certificate" \\
        -O "ssl" \\
        -f "dst port 7200"
Frame 39: 899 bytes on wire (7192 bits), 899 bytes captured (7192 bits) on interface 0
Internet Protocol Version 4, Src: 10.0.1.110, Dst: 10.0.1.109
Transmission Control Protocol, Src Port: 61918, Dst Port: 7200, Seq: 136, Ack: 916, Len: 843
Secure Sockets Layer
    TLSv1.2 Record Layer: Handshake Protocol: Certificate
        Version: TLS 1.2 (0x0303)
        Handshake Protocol: Certificate
          RDNSequence item: 1 item (id-at-commonName=Consul CA 7)
              RelativeDistinguishedName item (id-at-commonName=Consul CA 7)
                  Id: 2.5.4.3 (id-at-commonName)
                  DirectoryString: printableString (1)
                      printableString: Consul CA 7
          `}
                  />
                </div>
              </div>
            </div>
          </div>
        </section>

        <section class="g-section border-top large-padding">
          <div class="g-container">
            <div class="g-text-asset">
              <div>
                <div>
                  <h3 class="g-type-display-3">Mesh Gateway</h3>
                  <p class="g-type-body">
                    Connect between different cloud regions, VPCs and between
                    overlay and underlay networks without complex network
                    tunnels and NAT. Mesh Gateways solve routing at TLS layer
                    while preserving end-to-end encryption and limiting attack
                    surface area at the edge of each network.
                  </p>
                  <p>
                    <a
                      class="learn-more g-type-buttons-and-standalone-links"
                      href="/docs/connect/mesh_gateway.html"
                    >
                      Learn more
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="6"
                        height="10"
                        viewBox="0 0 6 10"
                      >
                        <g
                          fill="none"
                          fillRule="evenodd"
                          transform="translate(-6 -3)"
                        >
                          <mask id="a" fill="#fff">
                            <path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z" />
                          </mask>
                          <g fill="#1563FF" mask="url(#a)">
                            <path d="M0 0h16v16H0z" />
                          </g>
                        </g>
                      </svg>
                    </a>
                  </p>
                </div>
              </div>
              <div>
                <picture>
                  <img
                    src="/img/consul-connect/mesh-gateway/gateway_1200.png"
                    alt="Mesh gateway diagram"
                  />
                </picture>
              </div>
            </div>
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
