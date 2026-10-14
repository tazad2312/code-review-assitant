class ReviewMergeRequestJob < ApplicationJob
  queue_as :default

  def perform(review_run)
    review_run.with_lock do
      return if review_run.completed? || review_run.in_progress?

      review_run.update!(status: :in_progress)
    end

    begin
      repository = review_run.repository
      client = GitlabClient.new
      diff = client.merge_request_diff(repository.external_id, review_run.merge_request_iid)
      comments = InferenceClient.new.review(diff)

      review_run.transaction do
        review_run.review_comments.destroy_all
        comments.each do |comment|
          review_run.review_comments.create!(
            file_path: comment.fetch('file'),
            line_number: comment.fetch('line'),
            severity: comment.fetch('severity'),
            message: comment.fetch('message'),
            category: comment.fetch('category')
          )
        end
        review_run.update!(status: :completed, completed_at: Time.current)
      end

      if ENV['POST_COMMENTS_TO_MR'] == 'true'
        begin
          GitlabClient.new.post_review_notes(review_run, comments)
        rescue StandardError => e
          Rails.logger.error("Failed to post review notes for run #{review_run.id}: #{e.class}: #{e.message}")
        end
      end
    rescue StandardError => e
      review_run.update!(status: :failed)
      Rails.logger.error("Review run #{review_run.id} failed: #{e.class}: #{e.message}")
      raise
    end
  end
end
