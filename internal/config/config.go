package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type TableFilter struct {
	Include []string `yaml:"include"` // whitelist: only these tables (schema.table)
	Exclude []string `yaml:"exclude"` // blacklist: all except these (schema.table)
}

type DatabaseConfig struct {
	Name        string      `yaml:"name"`
	Host        string      `yaml:"host"`
	Port        int         `yaml:"port"`
	Database    string      `yaml:"database"`
	User        string      `yaml:"user"`
	PasswordEnv string      `yaml:"password_env"`
	SSLMode     string      `yaml:"sslmode"`
	MaxConns    int32       `yaml:"max_conns"`
	Schemas     []string    `yaml:"schemas"`
	Tables      TableFilter `yaml:"tables"`
}

// MaxPoolConns returns the configured max pool connections, defaulting to 5.
func (d *DatabaseConfig) MaxPoolConns() int32 {
	if d.MaxConns <= 0 {
		return 5
	}
	return d.MaxConns
}

type KnowledgeMapConfig struct {
	Path                  string `yaml:"path"`
	AutoDiscoverOnStartup bool   `yaml:"auto_discover_on_startup"`
}

type Config struct {
	Databases    []DatabaseConfig   `yaml:"databases"`
	KnowledgeMap KnowledgeMapConfig `yaml:"knowledgemap"`
}

// Password retrieves the database password from the environment variable
// specified in PasswordEnv. Returns an error if the variable is unset or empty.
func (d *DatabaseConfig) Password() (string, error) {
	val := os.Getenv(d.PasswordEnv)
	if val == "" {
		return "", fmt.Errorf("environment variable %q for database %q is not set or empty", d.PasswordEnv, d.Name)
	}
	return val, nil
}

// ConnString builds a PostgreSQL connection string with read-only mode enforced
// via the default_transaction_read_only runtime parameter (Tier 2 protection).
// All components are properly URL-encoded to prevent parameter injection.
func (d *DatabaseConfig) ConnString() (string, error) {
	password, err := d.Password()
	if err != nil {
		return "", err
	}
	sslmode := d.SSLMode
	if sslmode == "" {
		sslmode = "prefer"
	}
	switch sslmode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		// ok
	default:
		return "", fmt.Errorf("invalid sslmode %q for database %q", sslmode, d.Name)
	}
	port := d.Port
	if port == 0 {
		port = 5432
	}
	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", d.Host, port),
		Path:   "/" + d.Database,
	}
	u.User = url.UserPassword(d.User, password)
	q := u.Query()
	q.Set("sslmode", sslmode)
	q.Set("default_transaction_read_only", "on")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ShouldIncludeSchema reports whether the given schema should be included
// in discovery. Returns true if no schema filter is configured.
func (d *DatabaseConfig) ShouldIncludeSchema(schema string) bool {
	if len(d.Schemas) == 0 {
		return true
	}
	for _, s := range d.Schemas {
		if s == schema {
			return true
		}
	}
	return false
}

// ShouldIncludeTable reports whether the given schema.table should be
// included in discovery. Checks the include/exclude table filter.
// Returns true if no table filter is configured.
func (d *DatabaseConfig) ShouldIncludeTable(schema, table string) bool {
	qualified := schema + "." + table
	if len(d.Tables.Include) > 0 {
		for _, t := range d.Tables.Include {
			if t == qualified {
				return true
			}
		}
		return false
	}
	if len(d.Tables.Exclude) > 0 {
		for _, t := range d.Tables.Exclude {
			if t == qualified {
				return false
			}
		}
	}
	return true
}

// Load reads and validates a YAML configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Databases) == 0 {
		return fmt.Errorf("at least one database must be configured")
	}

	names := make(map[string]bool)
	for i, db := range c.Databases {
		if err := db.validate(i, names); err != nil {
			return err
		}
	}

	if c.KnowledgeMap.Path == "" {
		c.KnowledgeMap.Path = defaultKnowledgeMapPath()
	}

	return nil
}

func (d *DatabaseConfig) validate(index int, names map[string]bool) error {
	if d.Name == "" {
		return fmt.Errorf("database[%d]: name is required", index)
	}
	if names[d.Name] {
		return fmt.Errorf("duplicate database name: %q", d.Name)
	}
	names[d.Name] = true

	if d.Host == "" {
		return fmt.Errorf("database %q: host is required", d.Name)
	}
	if d.Database == "" {
		return fmt.Errorf("database %q: database is required", d.Name)
	}
	if d.User == "" {
		return fmt.Errorf("database %q: user is required", d.Name)
	}
	if d.PasswordEnv == "" {
		return fmt.Errorf("database %q: password_env is required", d.Name)
	}

	// Validate schema names
	for j, s := range d.Schemas {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("database %q: schemas[%d] is empty", d.Name, j)
		}
	}

	// Include and exclude are mutually exclusive
	if len(d.Tables.Include) > 0 && len(d.Tables.Exclude) > 0 {
		return fmt.Errorf("database %q: tables.include and tables.exclude are mutually exclusive", d.Name)
	}

	// Validate table entries are in schema.table format
	for j, t := range d.Tables.Include {
		if !strings.Contains(t, ".") {
			return fmt.Errorf("database %q: tables.include[%d] %q must be in schema.table format", d.Name, j, t)
		}
	}
	for j, t := range d.Tables.Exclude {
		if !strings.Contains(t, ".") {
			return fmt.Errorf("database %q: tables.exclude[%d] %q must be in schema.table format", d.Name, j, t)
		}
	}
	return nil
}

// defaultKnowledgeMapPath returns <UserConfigDir>/goro-pg/knowledgemap.db,
// falling back to ./knowledgemap.db if the config directory cannot be resolved.
// On Linux this is ~/.config/goro-pg/, on macOS ~/Library/Application Support/.
func defaultKnowledgeMapPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "./knowledgemap.db"
	}
	return filepath.Join(configDir, "goro-pg", "knowledgemap.db")
}
