require "active_storage"

module Thruster
  class Engine < ::Rails::Engine
    config.generators.api_only = true
  end
end
