class Thruster::ActiveStorage::InspectionsController < Thruster::BaseController
  def show
    representation = Thruster::ActiveStorage::Representation.find(params[:id], variation_key: params[:variation_key])

    if representation
      render json: representation.as_json
    else
      head :not_found
    end
  end
end
