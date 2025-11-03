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
  end
end
