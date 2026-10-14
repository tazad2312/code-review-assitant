class Repository < ApplicationRecord
  has_many :review_runs, dependent: :destroy

  validates :external_id, :name, presence: true
  validates :external_id, uniqueness: true
end
