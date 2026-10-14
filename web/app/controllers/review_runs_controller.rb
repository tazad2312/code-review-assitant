class ReviewRunsController < ApplicationController
  def index
    @review_runs = ReviewRun.includes(:repository).order(created_at: :desc)
  end

  def show
    @review_run = ReviewRun.includes(:repository, :review_comments).find(params[:id])
  end
end
