Rails.application.routes.draw do
  root 'review_runs#index'
  resources :review_runs, only: %i[index show]
  patch '/review_comments/:id/feedback', to: 'review_comments#feedback', as: :review_comment_feedback
  post '/webhooks/gitlab', to: 'webhooks/gitlab#create'
end
