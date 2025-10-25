require "thruster/version"
require "thruster/engine" if defined?(::Rails::Engine)

module Thruster
  def self.secret
    ENV["THRUSTER_SECRET"]
  end
end
