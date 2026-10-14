require 'rails_helper'

RSpec.describe ReviewComment, type: :model do
  it 'validates review fields and stores helpful feedback' do
    repository = Repository.create!(external_id: '42', name: 'example')
    run = repository.review_runs.create!(merge_request_iid: 5)
    comment = run.review_comments.create!(
      file_path: 'app.rb',
      line_number: 12,
      severity: 'high',
      message: 'Handle the error.',
      category: 'bug',
      feedback: :helpful
    )

    expect(comment).to be_valid
    expect(comment.feedback_helpful?).to be(true)
  end

  it 'rejects unsupported severities' do
    comment = described_class.new(file_path: 'app.rb', line_number: 1, severity: 'urgent',
                                  message: 'Message', category: 'bug')

    expect(comment).not_to be_valid
    expect(comment.errors[:severity]).to be_present
  end
end
