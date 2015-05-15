#-------------------------------------------------------------------------
# Configure Middleman
#-------------------------------------------------------------------------

set :base_url, "https://www.consul.io/"

activate :hashicorp do |h|
  h.version         = "0.5.1"
  h.bintray_enabled = ENV["BINTRAY_ENABLED"]
  h.bintray_repo    = "mitchellh/consul"
  h.bintray_user    = "mitchellh"
  h.bintray_key     = ENV["BINTRAY_API_KEY"]

  # Do not include the "web" in the default list of packages
  h.bintray_exclude_proc = Proc.new do |os, filename|
    os == "web"
  end

  # Consul packages are not prefixed with consul_ - they should be in the
  # future though!
  h.bintray_prefixed = false
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
