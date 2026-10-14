class CreateRepositories < ActiveRecord::Migration[7.1]
  def change
    create_table :repositories do |t|
      t.string :external_id, null: false
      t.string :name, null: false

      t.timestamps
    end
    add_index :repositories, :external_id, unique: true
  end
end
