Rails.application.routes.draw do
  namespace :thruster do
    namespace :active_storage do
      get "inspect/:id",
        to: "inspections#show",
        as: :inspection
    end
  end
end
