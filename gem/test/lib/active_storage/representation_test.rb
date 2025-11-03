require "test_helper"

class Thruster::ActiveStorage::RepresentationTest < ActiveSupport::TestCase
  test "find" do
    blob = active_storage_blobs(:thruster_png)
    signed_id = blob.signed_id

    representation = Thruster::ActiveStorage::Representation.find(signed_id)

    assert_not_nil representation
    assert_equal blob.id, representation.blob.id
    assert_nil representation.variation
    assert_equal false, representation.preview

    attachment = active_storage_attachments(:thruster_png)
    variant = attachment.variant(resize_to_limit: [ 100, 100 ])
    blob = variant.blob
    variation = variant.variation

    representation = Thruster::ActiveStorage::Representation.find(blob.signed_id, variation_key: variation.key)

    assert_not_nil representation
    assert_equal blob.id, representation.blob.id
    assert_not_nil representation.variation
    assert_equal variation.key, representation.variation.key
    assert_equal false, representation.preview

    attachment = active_storage_attachments(:thruster_mp4)
    preview_variant = attachment.preview(resize_to_limit: [ 100, 100 ])
    blob = preview_variant.blob
    variation = preview_variant.variation

    representation = Thruster::ActiveStorage::Representation.find(blob.signed_id, variation_key: variation.key)

    assert_not_nil representation
    assert_equal blob.id, representation.blob.id
    assert_not_nil representation.variation
    assert_equal true, representation.preview
  end

  test "find with invalid values" do
    representation = Thruster::ActiveStorage::Representation.find("invalid_signed_id")

    assert_nil representation
  end

  test "as_json" do
    blob = active_storage_blobs(:thruster_png)
    representation = Thruster::ActiveStorage::Representation.new(
      blob: blob,
      variation: nil,
      preview: false
    )

    json = representation.as_json

    assert_equal blob.key, json["key"]
    assert_equal blob.filename.to_s, json["filename"]
    assert_equal blob.content_type, json["content_type"]
    assert_equal blob.byte_size, json["byte_size"]
    assert_equal blob.checksum, json["checksum"]
    assert_equal blob.metadata, json["metadata"]
    assert_not_nil json["download_url"]
    assert_nil json["variation"]
    assert_nil json["preview"]

    attachment = active_storage_attachments(:thruster_png)
    variant = attachment.variant(resize_to_limit: [ 100, 100 ])
    blob = variant.blob
    variation = variant.variation

    representation = Thruster::ActiveStorage::Representation.new(
      blob: blob,
      variation: variation,
      preview: false
    )

    json = representation.as_json

    assert_not_nil json["variation"]
    assert_equal variation.as_json, json["variation"]
    assert_nil json["preview"]

    attachment = active_storage_attachments(:thruster_mp4)
    preview_variant = attachment.preview(resize_to_limit: [ 100, 100 ])
    blob = preview_variant.blob
    variation = preview_variant.variation

    representation = Thruster::ActiveStorage::Representation.new(
      blob: blob,
      variation: variation,
      preview: true
    )

    json = representation.as_json

    assert_equal blob.key, json["key"]
    assert_not_nil json["variation"]
    assert_not_nil json["preview"]
  end
end
