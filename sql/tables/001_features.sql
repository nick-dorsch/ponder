-- Features table to track which features are available/enabled
CREATE TABLE IF NOT EXISTS features (
  id CHAR(36) PRIMARY KEY,
  name VARCHAR(55) NOT NULL UNIQUE,
  description TEXT NOT NULL,
  specification TEXT NOT NULL,

  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS set_features_updated_at
AFTER UPDATE ON features
WHEN NEW.updated_at = OLD.updated_at
BEGIN
  UPDATE features
  SET updated_at = CURRENT_TIMESTAMP
  WHERE id = NEW.id;
END;

-- Seed the default feature (required for basic operation)
-- Note: id must be provided by the application (Go will generate UUIDs)
INSERT OR IGNORE INTO features (id, name, description, specification) VALUES
(
  '00000000-0000-0000-0000-000000000000',
  'misc',
  'Default feature for uncategorized tasks',
  'Use this feature in cases where a task is minimal and does not require a feature, such as minor hotfixes, tweaks etc.'
);
