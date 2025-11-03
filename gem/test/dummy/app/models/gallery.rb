class Gallery < ApplicationRecord
  has_many_attached :media

  validates :name, presence: true
end
