module Thruster::Authentication
  extend ActiveSupport::Concern

  included do
    before_action :require_authentication
  end

  private
    def require_authentication
      head :forbidden unless authenticated?
    end

    def authenticated?
      if Thruster.secret.blank?
        Rails.logger.warn "Thruster has no shared secret configured but someone tried to access its routes. " \
                          "Unauthenticated access is prohibited."
        false
      else
        authenticate_with_http_token do |token, options|
          ActiveSupport::SecurityUtils.secure_compare(token, Thruster.secret)
        end
      end
    end
end
