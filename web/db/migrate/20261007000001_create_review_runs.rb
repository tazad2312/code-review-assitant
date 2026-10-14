class CreateReviewRuns < ActiveRecord::Migration[7.1]
  def change
    create_table :review_runs do |t|
      t.references :repository, null: false, foreign_key: true
      t.integer :merge_request_iid, null: false
      t.string :commit_sha
      t.string :title
      t.text :merge_request_url
      t.integer :status, null: false, default: 0
      t.datetime :completed_at

      t.timestamps
    end
    add_index :review_runs, %i[repository_id merge_request_iid commit_sha],
              unique: true, name: 'index_review_runs_on_repository_mr_and_commit'
  end
end
