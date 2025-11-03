require "thruster/version"
require "thruster/engine" if defined?(::Rails::Engine)

module Thruster
  autoload :ActiveStorage, "thruster/active_storage"

  def self.secret
    ENV["THRUSTER_SECRET"]
  end
end
