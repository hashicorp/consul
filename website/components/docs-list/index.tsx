import Button from '@hashicorp/react-button'
import s from './style.module.css'
interface Doc {
  icon: {
    src: string
    alt: string
  }
  description: string
  cta: {
    text: string
    url: string
  }
}

interface DocsListProps {
  title: string
  docs: Doc[]
  className?: string
}

export default function DocsList({ title, docs, className }: DocsListProps) {
  return (
    <div className={className}>
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
