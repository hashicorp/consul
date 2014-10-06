#-------------------------------------------------------------------------
# Configure Middleman
#-------------------------------------------------------------------------

activate :hashicorp do |h|
  h.version      = '0.4.0'
  h.bintray_repo = 'mitchellh/consul'
  h.bintray_user = 'mitchellh'
  h.bintray_key  = ENV['BINTRAY_API_KEY']

  h.bintray_exclude_proc = Proc.new do |os, filename|
    os == 'web'
  end
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
end
