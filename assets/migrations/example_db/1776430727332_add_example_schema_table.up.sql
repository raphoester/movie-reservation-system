-- UP
CREATE SCHEMA example_schema;

CREATE TABLE example_schema.example_table (
    id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(255) NOT NULL
);
