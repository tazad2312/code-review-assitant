require 'rails_helper'

RSpec.describe ReviewRun, type: :model do
  it 'requires a positive merge request IID' do
    repository = Repository.create!(external_id: '42', name: 'example')
    run = repository.review_runs.new(merge_request_iid: 0)

    expect(run).not_to be_valid
    expect(run.errors[:merge_request_iid]).to be_present
  end

  it 'tracks the review lifecycle' do
    expect(described_class.statuses).to eq(
      'pending' => 0, 'in_progress' => 1, 'completed' => 2, 'failed' => 3
    )
  end
end
