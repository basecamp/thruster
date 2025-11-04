Rails.application.routes.draw do
  direct :thruster_active_storage do |model, options|
    representation = Thruster::ActiveStorage::Representation.new(model, **options)

    if representation.performs_transformations?
      representation.to_url
    else
      route_for(:rails_storage_proxy, model, options)
    end
  end
end
