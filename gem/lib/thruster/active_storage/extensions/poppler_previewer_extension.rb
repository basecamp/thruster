module Thruster::ActiveStorage::Extensions::PopplerPreviewerExtension
  extend ActiveSupport::Concern

  class_methods do
    def to_thruster_params
      {
        command: pdftoppm_path,
        arguments: [
          "-singlefile",
          "-cropbox",
          "-r", "72",
          "-png",
          Thruster::ActiveStorage::Extensions::INPUT_FILE_PATH_PLACEHOLDER
        ]
      }
    end
  end
end
