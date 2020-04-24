import InlineSvg from '@hashicorp/react-inline-svg'
import Image from '@hashicorp/react-image'
import Button from '@hashicorp/react-button'
import QuoteMarksIcon from './img/quote.svg?include'

export default function CaseStudySlide({
  caseStudy: { person, quote, company, caseStudyURL }
}) {
  return (
    <blockquote className="g-grid-container case-slide">
      <InlineSvg className="quotes" src={QuoteMarksIcon} />
      <h4 className="case g-type-display-4">{quote}</h4>
      <div className="case-content">
        <div className="person-container">
          <Image
            className="person-photo"
            url={person.photo}
            aspectRatio={[1, 1]}
            alt={`${person.firstName} ${person.lastName}`}
          />
          <div className="person-name">
            <h5 className="g-type-display-5">
              {person.firstName} {person.lastName}
            </h5>
            <p>
              {person.title}, {company.name}
            </p>
          </div>
        </div>
        <Image className="company-logo" url={company.logo} alt={company.name} />
      </div>
      <Button
        title="Read more"
        url={caseStudyURL}
        theme={{
          variant: 'tertiary',
          brand: 'nomad',
          background: 'light'
        }}
        linkType="outbound"
      />
    </blockquote>
  )
}
