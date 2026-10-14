class ReviewComment < ApplicationRecord
  belongs_to :review_run

  enum :feedback, { helpful: 0, not_helpful: 1 }, prefix: true

  validates :file_path, :message, :category, presence: true
  validates :line_number, numericality: { only_integer: true, greater_than: 0 }
  validates :severity, inclusion: { in: %w[critical high medium low] }
end
