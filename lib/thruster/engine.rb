module Thruster
  class Engine < ::Rails::Engine
    initializer "thruster.x_send_file" do
      if Rails.env.production?
        config.action_dispatch.x_sendfile_header = "X-Sendfile"
      end
    end
  end
end
