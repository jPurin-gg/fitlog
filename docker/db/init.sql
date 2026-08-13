-- The backend owns the schema through embedded, versioned migrations in
-- backend/internal/database/migrations. PostgreSQL runs this file only to keep
-- the Compose initialization mount explicit; no application tables belong here.
SELECT 1;
