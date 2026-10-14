module Webhooks
  class GitlabController < ActionController::API
    def create
      return head :unauthorized unless valid_token?

      event = JSON.parse(request.raw_post)
      return head :ok unless reviewable_event?(event)

      project = event.fetch('project')
      attributes = event.fetch('object_attributes')
      repository = Repository.find_or_initialize_by(external_id: project.fetch('id').to_s)
      repository.assign_attributes(name: project.fetch('name', project['path_with_namespace']))
      repository.save!

      commit_sha = attributes.dig('last_commit', 'id')
      run = repository.review_runs.find_or_create_by!(
        merge_request_iid: attributes.fetch('iid'),
        commit_sha: commit_sha
      ) do |record|
        record.title = attributes['title']
        record.merge_request_url = attributes['url']
      end
      ReviewMergeRequestJob.perform_later(run) if run.pending? || run.failed?
      head :accepted
    rescue JSON::ParserError, KeyError, ActiveRecord::RecordInvalid, ActiveRecord::RecordNotUnique => e
      Rails.logger.warn("Rejected GitLab webhook: #{e.class}: #{e.message}")
      head :unprocessable_content
    end

    private

    def valid_token?
      expected = ENV['GITLAB_WEBHOOK_SECRET'].to_s
      provided = request.headers['X-Gitlab-Token'].to_s
      expected.present? && expected.bytesize == provided.bytesize &&
        ActiveSupport::SecurityUtils.secure_compare(expected, provided)
    end

    def reviewable_event?(event)
      return false unless event['object_kind'] == 'merge_request'

      attributes = event['object_attributes'] || {}
      attributes['state'] == 'opened' && %w[open reopen update].include?(attributes['action'])
    end
  end
end
