require "test_helper"

class Thruster::ActiveStorage::Extensions::MuPDFPreviewerExtensionTest < ActiveSupport::TestCase
  test "to_thruster_params" do
    assert_respond_to ActiveStorage::Previewer::MuPDFPreviewer, :to_thruster_params

    params = ActiveStorage::Previewer::MuPDFPreviewer.to_thruster_params

    assert_kind_of Hash, params
    assert params.key?(:command)
    assert params.key?(:arguments)
  end
end
