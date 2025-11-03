module Thruster
  module ActiveStorage
    class Representation
      attr_reader :blob, :variation, :preview

      def self.find(blob_id, variation_key: nil)
        blob = ::ActiveStorage::Blob.find_signed(blob_id)

        if blob
          representation = blob.representation(variation_key) if variation_key

          new(
            blob: blob,
            variation: representation&.variation,
            preview: representation.is_a?(::ActiveStorage::Preview)
          )
        end
      end

      def initialize(blob:, variation:, preview:)
        @blob = blob
        @variation = variation
        @preview = preview
      end

      def as_json
        blob_data.merge(
          download_url: download_url,
          variation: variation_data,
          preview: preview_data,
        )
      end

      private
        def blob_data
          blob.as_json(only: %i[
            key
            filename
            content_type
            byte_size
            checksum
            metadata
          ])
        end

        def download_url
          Rails.application.routes.url_helpers.rails_blob_url(blob, only_path: true)
        end

        def variation_data
          variation&.as_json
        end

        def preview_data
          return unless preview

          {
            command: "foo",
            arguments: ""
          }
        end
    end
  end
end
