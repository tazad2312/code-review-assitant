class ReviewRun < ApplicationRecord
  belongs_to :repository
  has_many :review_comments, dependent: :destroy

  enum :status, { pending: 0, in_progress: 1, completed: 2, failed: 3 }, default: :pending

  validates :merge_request_iid, numericality: { only_integer: true, greater_than: 0 }
  validates :commit_sha, uniqueness: { scope: %i[repository_id merge_request_iid] }, allow_nil: true
end
