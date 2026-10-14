class ReviewCommentsController < ApplicationController
  def feedback
    comment = ReviewComment.find(params[:id])
    feedback = params.require(:feedback)
    return head :unprocessable_content unless ReviewComment.feedbacks.key?(feedback)

    comment.update!(feedback: feedback)
    redirect_back_or_to(review_run_path(comment.review_run), status: :see_other)
  end
end
