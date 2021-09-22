import s from './style.module.css'

interface Image {
  source: string
  alt: string
}

interface Block {
  title: string
  description: string
  image: Image
}

interface BlockListProps {
  blocks: Block[]
}

export default function BlockList({ blocks }: BlockListProps) {
  return (
    <div className={s.blocksContainer}>
      {blocks.map(({ image, title, description }) => (
        <div key={title} className={s.block}>
          <div className={s.imageContainer}>
            <img src={image} alt={title} />
          </div>
          <div>
            <h5 className={s.title}>{title}</h5>
            <p className={s.description}>{description}</p>
          </div>
        </div>
      ))}
    </div>
  )
}
