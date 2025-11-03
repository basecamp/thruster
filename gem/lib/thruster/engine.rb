require "active_storage"
require "thruster/active_storage/representation"

module Thruster
  class Engine < ::Rails::Engine
    config.generators.api_only = true
  end
end
