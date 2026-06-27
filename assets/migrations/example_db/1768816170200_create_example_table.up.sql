-- UP
CREATE TABLE example_table (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now()
)