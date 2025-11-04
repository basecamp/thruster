class Thruster::ActiveStorage::Representation
  attr_reader :model, :options

  def initialize(model, **options)
    @model = model
    @options = options
  end

  def to_url
    "/thruster/image_proxy/#{encrypt(compress(url_payload.to_json))}"
  end

  def performs_transformations?
    transformations&.any? || preview?
  end

  private
    def url_payload
      blob_data.merge(
        "download_url" => download_url,
        "transformations" => transformations,
        "preview" => preview
      )
    end

    def blob_data
      blob.as_json(only: %i[
        filename
        content_type
        byte_size
        checksum
      ])
    end

    def download_url
      Rails.application.routes.url_helpers.rails_storage_redirect_url(blob, only_path: true)
    end

    def transformations
      if model.respond_to?(:variation) && model.variation
        variation = model.variation
        variation.transformations.merge(format: variation.format)
      else
        nil
      end
    end

    def preview
      if preview? && previewer_class.respond_to?(:to_thruster_params)
        previewer_class.to_thruster_params
      else
        nil
      end
    end

    def preview?
      model.is_a?(::ActiveStorage::Preview)
    end

    def previewer_class
      @previewer_class ||= ::ActiveStorage.previewers.detect { |klass| klass.accept?(blob) }
    end

    def blob
      if model.respond_to?(:blob)
        model.blob
      elsif model.is_a?(ActiveStorage::Blob)
        model
      else
        raise ArgumentError, "The given model isn't a blob and can't be converted to a blob"
      end
    end

    def compress(data)
      Zlib::Deflate.deflate(data)
    end

    def encrypt(data)
      iv = generate_encryption_iv(data)

      cipher = OpenSSL::Cipher.new("aes-256-gcm")
      cipher.encrypt
      cipher.key = encryption_key
      cipher.iv = iv

      payload = [ cipher.update(data), cipher.final ]
      payload.unshift(cipher.auth_tag)
      payload.unshift(iv)

      Base64.urlsafe_encode64(payload.join(""), padding: false)
    end

    def encryption_key
      @encryption_key ||= ActiveSupport::KeyGenerator.new(Thruster.secret)
        .generate_key("thruster:active_storage:representation", 32)
    end

    def generate_encryption_iv(data)
      @encryption_id ||= begin
        iv_key = ActiveSupport::KeyGenerator.new(Thruster.secret)
          .generate_key("thruster:active_storage:representation:iv", 32)

        digest = OpenSSL::HMAC.digest("SHA256", iv_key, data)
        digest[0, 12]
      end
    end
end
