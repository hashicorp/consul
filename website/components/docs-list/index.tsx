import Button from '@hashicorp/react-button'
import s from './style.module.css'

interface Icon {
  src: string
  alt: string
}

interface Cta {
  text: string
  url: string
}

interface Doc {
  icon: Icon
  description: string
  cta: Cta
}

interface DocsListProps {
  title: string
  docs: Doc[]
}

export default function DocsList({ title, docs }: DocsListProps) {
  return (
    <div>
      <h3 className={s.title}>{title}</h3>
      <div className={s.docsList}>
        {docs.map(({ icon, description, cta }) => (
          <div key={cta.text}>
            <div className={s.image}>
              <img src={icon.src} alt={icon.alt} />
            </div>
            <p className={s.description}>{description}</p>
            <Button
              key="stable"
              url={cta.url}
              title={cta.text}
              linkType="inbound"
              theme={{
                variant: 'tertiary',
                brand: 'neutral',
              }}
            />
          </div>
        ))}
      </div>
    </div>
  )
}
