class CreateReviewComments < ActiveRecord::Migration[7.1]
  def change
    create_table :review_comments do |t|
      t.references :review_run, null: false, foreign_key: true
      t.string :file_path, null: false
      t.integer :line_number, null: false
      t.string :severity, null: false
      t.text :message, null: false
      t.string :category, null: false
      t.integer :feedback

      t.timestamps
    end
  end
end
