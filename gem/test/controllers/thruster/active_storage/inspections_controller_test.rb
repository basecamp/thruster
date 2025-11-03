require "test_helper"

class ActiveStorageIntrospectionControllerTest < ActionDispatch::IntegrationTest
  test "rejects unauthenticated requests" do
    get thruster_active_storage_inspection_path(id: :phony_id)
    assert_response :forbidden
  end

  test "rejects requests with invalid token" do
    get thruster_active_storage_inspection_path(id: :phony_id), headers: { "Authorization" => "Bearer invalid_token" }
    assert_response :forbidden
  end

  test "accepts authenticated requests" do
    get thruster_active_storage_inspection_path(id: :phony_id), headers: headers
    assert_response :not_found
  end

  test "returns metadata for a given blob" do
    attachment = active_storage_attachments(:thruster_png)
    attachment_id = attachment.signed_id

    get thruster_active_storage_inspection_path(id: attachment_id), headers: headers

    assert_response :ok
    assert response["Content-Type"].starts_with?("application/json")

    data = response.parsed_body

    assert_equal attachment.key, data["key"]
    assert_equal attachment.filename, data["filename"]
    assert_equal attachment.content_type, data["content_type"]
    assert_equal attachment.metadata.as_json, data["metadata"]
    assert_equal attachment.byte_size, data["byte_size"]
    assert_equal attachment.checksum, data["checksum"]

    assert_nil data["variation"]
  end

  test "returns metadata for a given variant" do
    variant = active_storage_attachments(:thruster_png).variant(resize_to_limit: [ 128, 128 ])
    blob = variant.blob
    variation = variant.variation

    get thruster_active_storage_inspection_path(id: blob.signed_id, variation_key: variation.key), headers: headers

    assert_response :ok
    assert response["Content-Type"].starts_with?("application/json")

    data = response.parsed_body

    assert_equal blob.key, data["key"]
    assert_equal blob.filename, data["filename"]
    assert_equal blob.content_type, data["content_type"]
    assert_equal blob.metadata.as_json, data["metadata"]
    assert_equal blob.byte_size, data["byte_size"]
    assert_equal blob.checksum, data["checksum"]

    assert_equal "png", data["variation"]["transformations"]["format"]
    assert_equal [ 128, 128 ], data["variation"]["transformations"]["resize_to_limit"]
  end

  test "returns metadata for a given preview" do
    variant = active_storage_attachments(:thruster_mp4).preview(format: :webp, resize_to_limit: [ 128, 128 ])
    blob = variant.blob
    variation = variant.variation

    get thruster_active_storage_inspection_path(id: blob.signed_id, variation_key: variation.key), headers: headers

    assert_response :ok
    assert response["Content-Type"].starts_with?("application/json")

    data = response.parsed_body

    assert_equal "webp", data["variation"]["transformations"]["format"]
    assert_equal [ 128, 128 ], data["variation"]["transformations"]["resize_to_limit"]
  end

  private
    def headers
      { "Authorization" => "Bearer #{Thruster.secret}" }
    end
end
