import Feature from './feature'
import s from './style.module.css'

export default function FeaturesList({ title, features }) {
  return (
    <div className={s.root}>
      <h2 className="g-type-display-1">{title}</h2>
      <div className={s.featuresContainer}>
        {features.map((feature, i) => (
          <Feature {...feature} number={i + 1} key={feature.title} />
        ))}
      </div>
    </div>
  )
}
