require "net/http"

$serf_files = {}
$serf_os = []

if ENV["SERF_VERSION"]
  raise "BINTRAY_API_KEY must be set." if !ENV["BINTRAY_API_KEY"]
  http = Net::HTTP.new("dl.bintray.com", 80)
  req = Net::HTTP::Get.new("/mitchellh/serf")
  req.basic_auth "mitchellh", ENV["BINTRAY_API_KEY"]
  response = http.request(req)

  response.body.split("\n").each do |line|
    next if line !~ /\/mitchellh\/serf\/(#{Regexp.quote(ENV["SERF_VERSION"])}.+?)'/
    filename = $1.to_s
    os = filename.split("_")[1]
    next if os == "SHA256SUMS"

    $serf_files[os] ||= []
    $serf_files[os] << filename
  end

  $serf_os = ["darwin", "linux", "windows"] & $serf_files.keys
  $serf_os += $serf_files.keys
  $serf_os.uniq!

  $serf_files.each do |key, value|
    value.sort!
  end
end

module DownloadHelpers
  def download_arch(file)
    parts = file.split("_")
    return "" if parts.length != 3
    parts[2].split(".")[0]
  end

  def download_os_human(os)
    if os == "darwin"
      return "Mac OS X"
    elsif os == "freebsd"
      return "FreeBSD"
    elsif os == "openbsd"
      return "OpenBSD"
    elsif os == "Linux"
      return "Linux"
    elsif os == "windows"
      return "Windows"
    else
      return os
    end
  end

  def download_url(file)
    "https://dl.bintray.com/mitchellh/serf/#{file}"
  end

  def latest_version
    ENV["SERF_VERSION"]
  end
end
