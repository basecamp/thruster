module Thruster
  def self.image_proxy_path(src)
    proxy_path = ENV["IMAGE_PROXY_PATH"]
    return src if proxy_path.nil?

    query = URI.encode_www_form({ src: src })
    "#{proxy_path}?#{query}"
  end
end
