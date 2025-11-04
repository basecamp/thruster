require "test_helper"

class Thruster::ActiveStorage::RepresentationTest < ActiveSupport::TestCase
  test "to_url" do
    attachment = active_storage_attachments(:thruster_mp4)
    preview = attachment.preview(
      **ActiveStorage.supported_image_processing_methods.map { |key| [key, SecureRandom.hex(16)] }.to_h.merge(format: :png)
    )
    representation = Thruster::ActiveStorage::Representation.new(preview)
    url = representation.to_url

    assert_kind_of String, url
    assert url.bytesize < 16.kilobytes
    assert_match %r{\A/thruster/image_proxy/[^/]+\Z}, url

    preview = attachment.preview(resize_to_limit: [ 512, 512 ])
    representation = Thruster::ActiveStorage::Representation.new(preview)
    first_url = representation.to_url

    preview = attachment.preview(resize_to_limit: [ 512, 512 ])
    representation = Thruster::ActiveStorage::Representation.new(preview)
    second_url = representation.to_url

    assert_equal first_url, second_url, "URLs should be stable for the same input"
  end

  test "performs_transformations?" do
    blob = active_storage_blobs(:thruster_png)
    representation = Thruster::ActiveStorage::Representation.new(blob)
    assert_not representation.performs_transformations?

    attachment = active_storage_attachments(:thruster_png)
    variant = attachment.variant(resize_to_limit: [ 100, 100 ])
    representation = Thruster::ActiveStorage::Representation.new(variant)
    assert representation.performs_transformations?

    attachment = active_storage_attachments(:thruster_mp4)
    preview = attachment.preview(resize_to_limit: [ 100, 100 ])
    representation = Thruster::ActiveStorage::Representation.new(preview)
    assert representation.performs_transformations?
  end
end
