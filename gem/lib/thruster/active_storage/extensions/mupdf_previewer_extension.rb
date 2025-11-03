module Thruster::ActiveStorage::Extensions::MuPDFPreviewerExtension
  extend ActiveSupport::Concern

  class_methods do
    def to_thruster_params
      {
        command: mutool_path,
        arguments: [
          "draw",
          "-F", "png",
          "-o", "-",
          Thruster::ActiveStorage::Extensions::INPUT_FILE_PATH_PLACEHOLDER,
          "1"
        ]
      }
    end
  end
end
