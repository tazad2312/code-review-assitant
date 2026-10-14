require 'rails_helper'

RSpec.describe 'GitLab webhook', type: :request do
  let(:payload) do
    {
      object_kind: 'merge_request',
      project: { id: 42, name: 'example', path_with_namespace: 'team/example' },
      object_attributes: {
        iid: 7,
        state: 'opened',
        action: 'open',
        title: 'Add feature',
        url: 'https://gitlab.example/team/example/-/merge_requests/7',
        last_commit: { id: 'deadbeef' }
      }
    }
  end

  before do
    allow(ENV).to receive(:[]).and_call_original
    allow(ENV).to receive(:[]).with('GITLAB_WEBHOOK_SECRET').and_return('webhook-secret')
  end

  it 'rejects requests without the shared secret' do
    post '/webhooks/gitlab', params: payload, as: :json

    expect(response).to have_http_status(:unauthorized)
    expect(Repository.count).to eq(0)
  end

  it 'creates a repository and queues a review for an opened merge request' do
    expect do
      post '/webhooks/gitlab',
           params: payload,
           headers: { 'X-Gitlab-Token' => 'webhook-secret' },
           as: :json
    end.to have_enqueued_job(ReviewMergeRequestJob)

    expect(response).to have_http_status(:accepted)
    expect(ReviewRun.last).to have_attributes(merge_request_iid: 7, commit_sha: 'deadbeef', status: 'pending')
  end

  it 'does not enqueue unrelated merge request actions' do
    payload[:object_attributes][:action] = 'merge'
    expect do
      post '/webhooks/gitlab',
           params: payload,
           headers: { 'X-Gitlab-Token' => 'webhook-secret' },
           as: :json
    end.not_to have_enqueued_job(ReviewMergeRequestJob)

    expect(response).to have_http_status(:ok)
  end
end
