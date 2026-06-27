package xpg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/lib/pq"
	"github.com/raphoester/movie-reservation-system/assets"
	"github.com/raphoester/movie-reservation-system/internal/shared/xcolls"
	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xruntime"
	"github.com/raphoester/movie-reservation-system/internal/shared/xtunnel"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "github.com/golang-migrate/migrate/v4/database/postgres" // register postgres driver for migrate
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/driver/postgres"

	sqltracer "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
)

// WithTunnelOpener configures an automatic SSM tunnel when connecting to a cloud database from a local machine.
// The tunnel is opened in Connect() and closed in Close().
func WithTunnelOpener(opener xtunnel.Opener) func(*params) {
	return func(p *params) { p.tunnelOpener = opener }
}

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Beginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

var (
	_ Querier  = (*Postgres)(nil)
	_ Beginner = (*Postgres)(nil)
)

// ErrDatabaseAlreadyExists is returned by CreateDatabase when the target database already exists.
var ErrDatabaseAlreadyExists = errors.New("database already exists")

type params struct {
	enableTracing bool
	tunnelOpener  xtunnel.Opener
}

func defaultParams() params {
	return params{
		enableTracing: false,
		tunnelOpener:  nil,
	}
}

func New(config Config, opts ...func(*params)) *Postgres {
	p := defaultParams()
	for _, opt := range opts {
		opt(&p)
	}

	return &Postgres{
		config:     config,
		params:     p,
		connected:  false,
		gormClient: nil,
		tunnel:     nil,
		sqlClient:  nil,
	}
}

func NewLazyLoaded(ctx context.Context, cfg LazyLoadedConfig, opts ...func(*params)) (*Postgres, error) {
	p := defaultParams()
	for _, opt := range opts {
		opt(&p)
	}

	config, err := cfg.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve postgres config: %w", err)
	}

	return &Postgres{
		config:     config,
		params:     p,
		connected:  false,
		gormClient: nil,
		sqlClient:  nil,
		tunnel:     nil,
	}, nil
}

func (p *Postgres) CreateDatabase(ctx context.Context, name string) error {
	if err := validateDbName(name); err != nil {
		return fmt.Errorf("invalid database name: %w", err)
	}

	quotedName := pq.QuoteIdentifier(name)
	if _, err := p.SQLClient().
		ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", quotedName)); err != nil {
		return fmt.Errorf("failed to create database: %w", mapCreateDatabaseError(err))
	}

	return nil
}

// duplicateDatabaseCode is the PostgreSQL error code for "database already exists".
const duplicateDatabaseCode = "42P04"

func mapCreateDatabaseError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == duplicateDatabaseCode {
		return ErrDatabaseAlreadyExists
	}

	return err
}

var allowedCharsInDBName = xcolls.NewSet(
	"_", "-",
	"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
)

func validateDbName(name string) error {
	if name == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	if len(name) > 63 {
		return fmt.Errorf("database name cannot exceed 63 characters")
	}

	for _, char := range name {
		if !allowedCharsInDBName.Contains(string(char)) {
			return fmt.Errorf("database name contains invalid character: %q", char)
		}
	}

	return nil
}

func validateSchemaName(name string) error {
	if name == "" {
		return fmt.Errorf("schema name cannot be empty")
	}

	if len(name) > 63 {
		return fmt.Errorf("schema name cannot exceed 63 characters")
	}

	for _, char := range name {
		if !allowedCharsInDBName.Contains(string(char)) {
			return fmt.Errorf("schema name contains invalid character: %q", char)
		}
	}

	return nil
}

// PoolConfig holds optional connection pool tuning parameters.
// All fields are pointers so that nil means "use the Go runtime default"
// rather than explicitly setting the value to zero.
type PoolConfig struct {
	MaxOpenConns    *int           `validate:"omitempty,min=1"`
	MaxIdleConns    *int           `validate:"omitempty,min=0"`
	ConnMaxLifetime *time.Duration `validate:"omitempty"`
	ConnMaxIdleTime *time.Duration `validate:"omitempty"`
}

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	Schema   string
	Pool     PoolConfig
}

type LazyLoadedConfig struct {
	Host     xconfigs.LazyString `validate:"required"`
	Port     xconfigs.LazyString `validate:"required"`
	User     xconfigs.LazyString `validate:"required"`
	Password xconfigs.LazyString `validate:"required"`
	DBName   xconfigs.LazyString `validate:"required"`
	SSLMode  xconfigs.LazyString `validate:"required"`
	Schema   string
	Pool     PoolConfig
}

func (c LazyLoadedConfig) Resolve(ctx context.Context) (Config, error) {
	var trueErrs []error
	var missingVals []string
	resolve := func(key string, ls xconfigs.LazyString) string {
		value, err := ls.Value(ctx)
		if err != nil {
			if errors.Is(err, xconfigs.ErrLazyStringNotInitialized) {
				missingVals = append(missingVals, key)
			} else {
				trueErrs = append(trueErrs, fmt.Errorf("failed to resolve lazy string %q: %w", key, err))
			}
		}

		return value
	}

	ret := Config{
		Host:     resolve("host", c.Host),
		Port:     resolve("port", c.Port),
		User:     resolve("user", c.User),
		Password: resolve("password", c.Password),
		DBName:   resolve("dbname", c.DBName),
		SSLMode:  resolve("sslmode", c.SSLMode),
		Schema:   c.Schema,
		Pool:     c.Pool,
	}

	if len(trueErrs) > 0 {
		return Config{}, fmt.Errorf("encountered errors resolving lazy config: %v", trueErrs)
	}

	if len(missingVals) > 0 {
		return Config{}, fmt.Errorf("missing required lazy config values: %v", missingVals)
	}

	return ret, nil
}

type Postgres struct {
	config Config
	params params

	connected  bool
	gormClient *gorm.DB
	sqlClient  *sql.DB
	tunnel     xtunnel.Tunnel
}

func (p *Postgres) DBName() string {
	return p.config.DBName
}

func (p *Postgres) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	tx, err := p.sqlClient.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

func (p *Postgres) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := p.sqlClient.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to exec query: %w", err)
	}
	return result, nil
}

func (p *Postgres) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	rows, err := p.sqlClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	return rows, nil
}

func (p *Postgres) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.sqlClient.QueryRowContext(ctx, query, args...)
}

func (p *Postgres) dsn() string {
	base := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		p.config.Host,
		p.config.Port,
		p.config.User,
		p.config.Password,
		p.config.DBName,
		p.config.SSLMode,
	)
	if p.config.Schema != "" {
		base += fmt.Sprintf(" search_path=%s", p.config.Schema)
	}
	return base
}

func (p *Postgres) url() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(p.config.User, p.config.Password),
		Host:   fmt.Sprintf("%s:%s", p.config.Host, p.config.Port),
		Path:   "/" + p.config.DBName,
	}

	q := u.Query()
	q.Set("sslmode", p.config.SSLMode)
	if p.config.Schema != "" {
		q.Set("search_path", p.config.Schema)
	}
	u.RawQuery = q.Encode()

	return u.String()
}

func applyPoolConfig(db *sql.DB, cfg PoolConfig) {
	if cfg.MaxOpenConns != nil {
		db.SetMaxOpenConns(*cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != nil {
		db.SetMaxIdleConns(*cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != nil {
		db.SetConnMaxLifetime(*cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != nil {
		db.SetConnMaxIdleTime(*cfg.ConnMaxIdleTime)
	}
}

func (p *Postgres) openTunnelIfNeeded(ctx context.Context) error {
	if p.params.tunnelOpener == nil {
		return nil
	}
	rt := xruntime.Detect(ctx)
	if rt.IsCloudDeployment {
		return nil
	}

	remotePort, err := strconv.Atoi(p.config.Port)
	if err != nil {
		return fmt.Errorf("invalid postgres port %q: %w", p.config.Port, err)
	}

	t, err := p.params.tunnelOpener.OpenTunnel(ctx, p.config.Host, remotePort)
	if err != nil {
		return fmt.Errorf("open SSM tunnel to postgres: %w", err)
	}
	if t.LocalPort() == -1 {
		// nil or zero-port tunnel means the opener decided no tunnel is needed (e.g. Nopener).
		return nil
	}

	p.tunnel = t
	p.config.Host = "localhost"
	p.config.Port = strconv.Itoa(t.LocalPort())

	return nil
}

func (p *Postgres) ConnectCtx(ctx context.Context) (retErr error) {
	if p.config.Schema != "" {
		if err := validateSchemaName(p.config.Schema); err != nil {
			return fmt.Errorf("invalid schema: %w", err)
		}
	}

	if err := p.openTunnelIfNeeded(ctx); err != nil {
		return err
	}
	// If anything below fails, close the tunnel so we don't leak the background process.
	defer func() {
		if retErr != nil && p.tunnel != nil {
			_ = p.tunnel.Close()
			p.tunnel = nil
		}
	}()

	openSQL := sql.Open
	if p.params.enableTracing {
		openSQL = func(driver, dsn string) (*sql.DB, error) {
			return sqltracer.Open(
				driver,
				dsn,
				// service name is not necessary if only one database is used in the application
			)
		}
	}

	sqlDB, err := openSQL("postgres", p.dsn())
	if err != nil {
		return fmt.Errorf("open sql: %w", err)
	}

	applyPoolConfig(sqlDB, p.config.Pool)

	gormDB, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		},
	)
	if err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("failed to open gorm: %w", err)
	}

	// Verify the connection is actually reachable. gorm.Open is lazy — without an
	// explicit ping, a broken tunnel or unreachable host goes undetected until the
	// first query. A 30-second deadline is enough for SSM tunnel + SSL + auth.
	pingCtx, cancelPing := context.WithTimeout(ctx, 30*time.Second)
	defer cancelPing()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}

	p.sqlClient = sqlDB
	p.gormClient = gormDB
	p.connected = true

	return nil
}

// Connect is a convenience wrapper around ConnectCtx with a background context.
//
// Deprecated: use ConnectCtx with an explicit context instead to allow for cancellation and timeouts.
func (p *Postgres) Connect() error {
	return p.ConnectCtx(context.Background())
}

func (p *Postgres) SQLClient() *sql.DB {
	if !p.connected {
		panic("postgres not connected")
	}

	return p.sqlClient
}

func (p *Postgres) GormClient() *gorm.DB {
	if !p.connected {
		panic("postgres not connected")
	}

	return p.gormClient
}

// withGormDB returns a new Postgres instance backed by an existing *gorm.DB.
// Used internally to create transaction-scoped instances via RunInTransaction.
func (p *Postgres) withGormDB(db *gorm.DB) *Postgres {
	return &Postgres{
		config:     p.config,
		params:     p.params,
		connected:  true,
		tunnel:     nil,
		gormClient: db,
		sqlClient:  p.sqlClient,
	}
}

// RunInTransaction executes fn inside a single GORM transaction.
// fn receives a *Postgres whose GormClient() is bound to the transaction.
// If fn returns an error the transaction is rolled back; otherwise committed.
func (p *Postgres) RunInTransaction(ctx context.Context, fn func(tx *Postgres) error) error {
	if err := p.GormClient().WithContext(ctx).Transaction(func(gormTx *gorm.DB) error {
		if err := fn(p.withGormDB(gormTx)); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}
	return nil
}

func (p *Postgres) Close() error {
	var closeTunnelErr error
	if p.tunnel != nil {
		closeTunnelErr = p.tunnel.Close()
		p.tunnel = nil
	}

	var closeSQLErr error
	if p.sqlClient != nil {
		closeSQLErr = p.sqlClient.Close()
	}

	return errors.Join(closeTunnelErr, closeSQLErr)
}

func (p *Postgres) Purge(ctx context.Context) error {
	if !p.connected {
		return errors.New("postgres not connected")
	}

	tables, err := p.listTruncatableTables(ctx)
	if err != nil {
		return err
	}

	if len(tables) == 0 {
		return nil
	}

	tx := p.gormClient.WithContext(ctx).Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", strings.Join(tables, ",")))
	if tx.Error != nil {
		return fmt.Errorf("failed to truncate database: %w", tx.Error)
	}

	return nil
}

// PurgeExcept truncates all user tables except those listed in excludedTables.
// Use this for tables that hold static reference data seeded by migrations and
// must survive between test runs (e.g. lookup tables).
func (p *Postgres) PurgeExcept(ctx context.Context, excludedTables ...string) error {
	if !p.connected {
		return errors.New("postgres not connected")
	}

	if len(excludedTables) == 0 {
		return p.Purge(ctx)
	}

	all, err := p.listTruncatableTables(ctx)
	if err != nil {
		return err
	}

	excluded := xcolls.NewSet(excludedTables...)
	tables := make([]string, 0, len(all))
	for _, t := range all {
		// all entries are already "schema.name" qualified; extract the name portion for the set lookup.
		_, name, _ := strings.Cut(t, ".")
		if !excluded.Contains(name) {
			tables = append(tables, t)
		}
	}

	if len(tables) == 0 {
		return nil
	}

	if err := p.gormClient.WithContext(ctx).Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", strings.Join(tables, ","))).Error; err != nil {
		return fmt.Errorf("failed to truncate database: %w", err)
	}

	return nil
}

// listTruncatableTables returns all user-created base tables as "schema.name" strings,
// excluding built-in Postgres schemas and tables owned by extensions.
//
// table_type = 'BASE TABLE' excludes views (e.g. PostGIS registers geometry_columns and
// geography_columns as views, which cannot be truncated).
// The schema filter excludes built-in Postgres schemas; all user-created schemas
// (including non-public ones like hotel_profile) are included.
// The NOT EXISTS subquery excludes tables owned by extensions via pg_depend deptype='e'
// (e.g. PostGIS's spatial_ref_sys, which holds coordinate reference system definitions
// and must not be wiped). This is the canonical way to identify extension-owned objects
// without hardcoding names.
func (p *Postgres) listTruncatableTables(ctx context.Context) ([]string, error) {
	type tableRef struct {
		TableSchema string
		TableName   string
	}

	var refs []tableRef
	if err := p.gormClient.WithContext(ctx).Raw(`
		SELECT t.table_schema, t.table_name
		FROM information_schema.tables t
		WHERE t.table_schema NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		  AND t.table_type = 'BASE TABLE'
		  AND NOT EXISTS (
		      SELECT 1
		      FROM pg_class c
		      JOIN pg_depend d ON c.oid = d.objid AND d.deptype = 'e'
		      WHERE c.relname = t.table_name
		        AND c.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = t.table_schema)
		  )
	`).Scan(&refs).Error; err != nil {
		return nil, fmt.Errorf("failed to list tables for truncation: %w", err)
	}

	tables := make([]string, 0, len(refs))
	for _, r := range refs {
		tables = append(tables, r.TableSchema+"."+r.TableName)
	}
	return tables, nil
}

func (p *Postgres) Migrate(migrationsDir string) error {
	if p.config.Schema != "" {
		if err := p.Connect(); err != nil {
			return fmt.Errorf("failed to connect for schema creation: %w", err)
		}
		if err := p.gormClient.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pq.QuoteIdentifier(p.config.Schema))).Error; err != nil {
			return fmt.Errorf("failed to create schema %q: %w", p.config.Schema, err)
		}
	}

	ioFs, err := iofs.New(assets.Migrations(), migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to create iofs source: %w", err)
	}
	defer func() { _ = ioFs.Close() }()

	m, err := migrate.NewWithSourceInstance("iofs", ioFs, p.url())
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
