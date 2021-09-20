import s from './style.module.css'

export default function BlockList({ blocks }) {
  return (
    <div className={s.blocksContainer}>
      {blocks.map(({ image, title, description }) => (
        <div key={title} className={s.block}>
          <img src={image} alt={title} />
          <h5 className="g-type-display-5">{title}</h5>
          <p>{description}</p>
        </div>
      ))}
    </div>
  )
}
