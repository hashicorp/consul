set :base_url, "https://www.consul.io/"

activate :hashicorp do |h|
  h.name        = "consul"
  h.version     = "1.5.2"
  h.github_slug = "hashicorp/consul"
end

# Netlify redirects/headers
proxy '_redirects', 'redirects.txt', ignore: true

helpers do
  # Returns a segment tracking ID such that local development is not
  # tracked to production systems.
  def segmentId()
    if (ENV['ENV'] == 'production')
      'IyzLrqXkox5KJ8XL4fo8vTYNGfiKlTCm'
    else
      '0EXTgkNx0Ydje2PGXVbRhpKKoe5wtzcE'
    end
  end

  # Returns the FQDN of the image URL.
  #
  # @param [String] path
  #
  # @return [String]
  def image_url(path)
    File.join(base_url, image_path(path))
  end

  # Get the title for the page.
  #
  # @param [Middleman::Page] page
  #
  # @return [String]
  def title_for(page)
    if page && page.data.page_title
      return "#{page.data.page_title} - Consul by HashiCorp"
    end

     "Consul by HashiCorp"
   end

  # Get the description for the page
  #
  # @param [Middleman::Page] page
  #
  # @return [String]
  def description_for(page)
    description = (page.data.description || "Consul by HashiCorp")
      .gsub('"', '')
      .gsub(/\n+/, ' ')
      .squeeze(' ')

    return escape_html(description)
  end

  # This helps by setting the "active" class for sidebar nav elements
  # if the YAML frontmatter matches the expected value.
  def sidebar_current(expected)
    current = current_page.data.sidebar_current || ""
    if current.start_with?(expected)
      return " class=\"active\""
    else
      return ""
    end
  end

  # Returns the id for this page.
  # @return [String]
  def body_id_for(page)
    if !(name = page.data.sidebar_current).blank?
      return "page-#{name.strip}"
    end
    if page.url == "/" || page.url == "/index.html"
      return "page-home"
    end
    if !(title = page.data.page_title).blank?
      return title
        .downcase
        .gsub('"', '')
        .gsub(/[^\w]+/, '-')
        .gsub(/_+/, '-')
        .squeeze('-')
        .squeeze(' ')
    end
    return ""
  end

  # Returns the list of classes for this page.
  # @return [String]
  def body_classes_for(page)
    classes = []

    if !(layout = page.data.layout).blank?
      classes << "layout-#{page.data.layout}"
    end

    if !(title = page.data.page_title).blank?
      title = title
        .downcase
        .gsub('"', '')
        .gsub(/[^\w]+/, '-')
        .gsub(/_+/, '-')
        .squeeze('-')
        .squeeze(' ')
      classes << "page-#{title}"
    end

    return classes.join(" ")
  end
end
