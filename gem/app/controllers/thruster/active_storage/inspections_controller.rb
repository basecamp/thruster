class Thruster::ActiveStorage::InspectionsController < Thruster::BaseController
  def show
    blob = ActiveStorage::Blob.find_signed(params[:id])

    if blob
      variation = blob.representation(params[:variation_key])&.variation if params.key?(:variation_key)
      service = ActiveStorage::Blob.services.fetch(blob.service_name)

      render json: {
        blob: {
          key: blob.key,
          filename: blob.filename,
          content_type: blob.content_type,
          metadata: blob.metadata,
          byte_size: blob.byte_size,
          checksum: blob.checksum,
        },
        variation: variation&.as_json,
        service: {
          type: service.send(:service_name),
          config: service.as_json
        }
      }
    else
      head :not_found
    end
  end
end
