import Button from '@hashicorp/react-button'

export default function CloudOfferingsList({ offerings }) {
  return (
    <ul className="g-cloud-offerings-list">
      {offerings.map((offering) => (
        <li key={offering.title}>
          <a
            href={offering.link.url}
            rel={offering.link.type === 'outbound' ? 'noopener' : undefined}
            target={offering.link.type === 'outbound' ? '_blank' : undefined}
          >
            <img src={offering.image} alt={offering.title} />
            <span className="g-type-label-strong">{offering.eyebrow}</span>
            <h4 className="g-type-display-4">{offering.title}</h4>
            <p>{offering.description}</p>
            <Button
              title={offering.link.text}
              linkType={offering.link.type}
              theme={{ variant: 'tertiary', brand: 'consul' }}
              url={offering.link.url}
            />
          </a>
        </li>
      ))}
    </ul>
  )
}
