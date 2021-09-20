export default function BlockList({ blocks }) {
  return (
    <div>
      {blocks.map(({ image, title, description }) => (
        <div key={title}>
          <img src={image} alt={title} />
          <h5 className="g-type-display-5">{title}</h5>
          <p>{description}</p>
        </div>
      ))}
    </div>
  )
}
