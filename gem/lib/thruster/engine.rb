require "active_storage"

module Thruster
  class Engine < ::Rails::Engine
    config.generators.api_only = true

    config.to_prepare do
      ::ActiveStorage::Previewer::VideoPreviewer.include(
        Thruster::ActiveStorage::Extensions::VideoPreviewerExtension
      )
      ::ActiveStorage::Previewer::MuPDFPreviewer.include(
        Thruster::ActiveStorage::Extensions::MuPDFPreviewerExtension
      )
      ::ActiveStorage::Previewer::PopplerPDFPreviewer.include(
        Thruster::ActiveStorage::Extensions::PopplerPreviewerExtension
      )
    end

    initializer "thruster.active_storage_configuration", before: "active_storage.configs" do
      config.before_initialize do |app|
        if Thruster.active_storage_integration_enabled?
          app.config.active_storage.resolve_model_to_route ||= :thruster_active_storage
        end
      end
    end
  end
end
