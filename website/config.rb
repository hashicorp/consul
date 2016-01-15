#-------------------------------------------------------------------------
# Configure Middleman
#-------------------------------------------------------------------------

set :base_url, "https://www.consul.io/"

activate :hashicorp do |h|
  h.name        = "consul"
  h.version     = "0.6.3"
  h.github_slug = "hashicorp/consul"
end

helpers do
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
end
