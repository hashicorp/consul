function BeforeAfterDiagram({
  beforeHeading,
  beforeSubTitle,
  beforeImage,
  beforeDescription,
  afterHeading,
  afterSubTitle,
  afterImage,
  afterDescription,
}) {
  return (
    <div className="g-timeline">
      <div>
        <span className="line"></span>
        <span className="line">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="11"
            height="15"
            viewBox="0 0 11 15"
          >
            <path
              fill="#CA2171"
              d="M0 0v15l5.499-3.751L11 7.5 5.499 3.749.002 0z"
            />
          </svg>
        </span>
        <span className="dot"></span>
        <h3>{beforeHeading}</h3>
        <span className="sub-heading">{beforeSubTitle}</span>
        <img
          src={beforeImage}
          alt={beforeSubTitle}
          className="static-callout"
        />
        {beforeDescription && <p>{beforeDescription}</p>}
      </div>
      <div>
        <span className="dot"></span>
        <h3>{afterHeading}</h3>
        <span className="sub-heading">{afterSubTitle}</span>
        <div id="index-dynamic-animation">
          <img
            src={afterImage}
            alt={afterSubTitle}
            className="static-callout"
          />
        </div>
        {afterDescription && <p>{afterDescription}</p>}
      </div>
    </div>
  )
}

export default BeforeAfterDiagram
