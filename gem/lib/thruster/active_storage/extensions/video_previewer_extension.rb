module Thruster::ActiveStorage::Extensions::VideoPreviewerExtension
  extend ActiveSupport::Concern

  class_methods do
    def to_thruster_params
      {
        command: ffmpeg_path,
        arguments: [
          "-i",
          Thruster::ActiveStorage::Extensions::INPUT_FILE_PATH_PLACEHOLDER,
          *Shellwords.split(ActiveStorage.video_preview_arguments),
          "-"
        ]
      }
    end
  end
end
