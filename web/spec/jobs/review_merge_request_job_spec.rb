require 'rails_helper'

RSpec.describe ReviewMergeRequestJob, type: :job do
  let(:repository) { Repository.create!(external_id: '42', name: 'example') }
  let(:review_run) { repository.review_runs.create!(merge_request_iid: 5, commit_sha: 'abc123') }
  let(:gitlab_url) { 'https://gitlab.example.test/api/v4/projects/42/merge_requests/5/diffs' }
  let(:inference_url) { 'http://inference.example.test/v1/review' }
  let(:diff_response) do
    [{ old_path: 'app.rb', new_path: 'app.rb', diff: "@@ -1 +1 @@\n-old\n+new\n" }]
  end

  before do
    stub_request(:get, gitlab_url)
      .with(headers: { 'PRIVATE-TOKEN' => 'gitlab-test-token' })
      .to_return(status: 200, body: JSON.generate(diff_response), headers: { 'Content-Type' => 'application/json' })
    stub_request(:post, inference_url)
      .with do |request|
        JSON.parse(request.body).fetch('diff').include?('diff --git a/app.rb b/app.rb')
      end
      .to_return(
        status: 200,
        body: JSON.generate(comments: [
                              { file: 'app.rb', line: 1, severity: 'high', message: 'Handle this case.',
                                category: 'bug' }
                            ]),
        headers: { 'Content-Type' => 'application/json' }
      )
    allow(ENV).to receive(:fetch).and_call_original
    allow(ENV).to receive(:fetch).with('GITLAB_API_URL', anything).and_return('https://gitlab.example.test/api/v4')
    allow(ENV).to receive(:fetch).with('INFERENCE_URL', anything).and_return('http://inference.example.test')
    allow(ENV).to receive(:[]).and_call_original
    allow(ENV).to receive(:[]).with('GITLAB_API_TOKEN').and_return('gitlab-test-token')
  end

  it 'fetches the diff, saves comments, and completes the run' do
    described_class.perform_now(review_run)

    expect(review_run.reload).to be_completed
    expect(review_run.review_comments.count).to eq(1)
    expect(review_run.review_comments.first).to have_attributes(
      file_path: 'app.rb', line_number: 1, severity: 'high', category: 'bug'
    )
  end

  it 'marks the run failed and surfaces inference errors' do
    stub_request(:post, inference_url).to_return(status: 502, body: 'unavailable')

    expect { described_class.perform_now(review_run) }.to raise_error(/Inference service request failed/)
    expect(review_run.reload).to be_failed
  end
end
