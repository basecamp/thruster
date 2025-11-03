module Thruster::ActiveStorage::Extensions
  INPUT_FILE_PATH_PLACEHOLDER = "{thruster_input_file_path}"

  autoload :MuPDFPreviewerExtension, "thruster/active_storage/extensions/mupdf_previewer_extension"
  autoload :PopplerPreviewerExtension, "thruster/active_storage/extensions/poppler_previewer_extension"
  autoload :VideoPreviewerExtension, "thruster/active_storage/extensions/video_previewer_extension"
end
