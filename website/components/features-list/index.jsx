import Feature from './feature'
import s from './style.module.css'

export default function FeaturesList({ title, features }) {
  return (
    <div
      className={s.featureListContainer}
      style={{
        backgroundImage: `url(${require('./images/top-right-design.svg')})`,
      }}
    >
      <div className="g-grid-container">
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
