import Feature from './feature'
import s from './style.module.css'
import { FeatureProps } from './feature'

interface FeaturesListProps {
  title: string
  features: Omit<FeatureProps, 'number'>[]
}

export default function FeaturesList({ title, features }: FeaturesListProps) {
  return (
    <div
      className={s.featureListContainer}
      style={{
        backgroundImage: `url(${require('./images/top-right-design.svg')}), url(${require('./images/bottom-left-design.svg')})`,
      }}
    >
      <div className={s.contentWrapper}>
        <h2 className={s.title}>{title}</h2>
        <div className={s.featuresContainer}>
          {features.map((feature, i) => (
            <Feature {...feature} number={i + 1} key={feature.title} />
          ))}
        </div>
      </div>
    </div>
  )
}
