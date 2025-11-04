require "thruster/version"
require "thruster/engine" if defined?(::Rails::Engine)

module Thruster
  autoload :ActiveStorage, "thruster/active_storage"

  class << self
    def secret
      ENV["THRUSTER_SECRET"]
    end

    def active_storage_integration_enabled?
      secret && ENV["IMAGE_PROXY_ENABLED"]
    end
  end
end
