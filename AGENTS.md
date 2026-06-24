# This Project
This is a Loyalty Points Wallet.
It's purpose is to capture loyalty points transactions for a user/member.

## Database Driven / Data first

- The database design and source is stored in sql/model/database_model.dbml
- Always change the database models first and look at how they effect the rest of the system.
- To regenerate the models in the internal/models from sql/model run this ```
    relspec convert --from dbml --from-path ./sql/model/database_model.dbml --to gorm --to-path ./internal/models --package models --types stdlib

  ```


