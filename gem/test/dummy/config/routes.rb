Rails.application.routes.draw do
  get "up" => "rails/health#show", as: :rails_health_check

  resources :galleries, only: :show
  root "galleries#index"
end
