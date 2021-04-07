import Button from '@hashicorp/react-button'

/** TODO: this should use the `react-components` version — https://react-components.vercel.app/?component=LearnCallout */

export default function LearnNomad({ headline, brand, items }) {
  return (
    <div className={`g-learn-callout`}>
      <div className="g-grid-container learn-container">
        <div className="column-container">
          {/* need this wrapper to flex center the .column-content */}
          <div>
            <div className="column-content">
              <h2 className="g-type-display-2">{headline}</h2>
              <Button
                className="desktop-button"
                title="Explore HashiCorp Learn"
                url={`https://learn.hashicorp.com/${brand}`}
                linkType="outbound"
                theme={{ variant: 'primary', brand }}
              />
            </div>
          </div>
          {items.map((item) => {
            return (
              <a
                key={item.title}
                href={item.link}
                target="_blank"
                rel="noopener noreferrer"
              >
                <div className="course">
                  <div className="image">
                    <div className="g-type-label-strong time">{item.time}</div>
                    <img src={item.image} alt={item.title} />
                  </div>
                  <div className="content">
                    <div>
                      <label className="g-type-label-strong category">
                        {item.category}
                      </label>
                      <h4 className="g-type-display-4">{item.title}</h4>
                    </div>
                  </div>
                </div>
              </a>
            )
          })}
        </div>
        <Button
          className="mobile-button"
          title="Explore HashiCorp Learn"
          url={`https://learn.hashicorp.com/${brand}`}
          linkType="outbound"
          theme={{ variant: 'primary', brand }}
        />
      </div>
    </div>
  )
}
