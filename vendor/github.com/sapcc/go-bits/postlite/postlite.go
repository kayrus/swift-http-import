/*******************************************************************************
*
* Copyright 2018 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

//Package postlite is a database library for applications that use PostgreSQL
//in production and in-memory SQLite for testing. It imports the necessary SQL
//drivers and integrates github.com/golang-migrate/migrate for data definition.
//When running with SQLite, executed SQL statements are logged with
//logg.Debug() from github.com/sapcc/go-bits/logg.
package postlite

import (
	"database/sql"
	"errors"
	"fmt"
	net_url "net/url"
	"os"
	"regexp"
	"strings"

	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database"
	"github.com/golang-migrate/migrate/database/postgres"
	"github.com/golang-migrate/migrate/database/sqlite3"
	"github.com/golang-migrate/migrate/source"
	bindata "github.com/golang-migrate/migrate/source/go_bindata"

	//enable postgres driver for database/sql
	_ "github.com/lib/pq"
)

//Configuration contains settings for Init(). The field Migrations needs to have keys
//matching the filename format expected by github.com/golang-migrate/migrate
//(see documentation there for details), for example:
//
//    cfg.Migrations = map[string]string{
//        "001_initial.up.sql": `
//            CREATE TABLE things (
//                id   BIGSERIAL NOT NULL PRIMARY KEY,
//                name TEXT NOT NULL,
//            );
//        `,
//        "001_initial.down.sql": `
//            DROP TABLE things;
//        `,
//    }
//
type Configuration struct {
	//(required for Postgres, ignored for SQLite) A libpq connection URL, see:
	//<https://www.postgresql.org/docs/9.6/static/libpq-connect.html#LIBPQ-CONNSTRING>
	PostgresURL *net_url.URL
	//(required) The schema migrations, in Postgres syntax. See above for details.
	Migrations map[string]string
	//(optional) If not empty, use this database/sql driver instead of "postgres"
	//or "sqlite3-postlite". This is useful e.g. when using github.com/majewsky/sqlproxy.
	OverrideDriverName string
}

//Connect connects to a Postgres database if cfg.PostgresURL is set, or to an
//in-memory SQLite3 database otherwise. Use of SQLite3 is only safe in unit
//tests! Unit tests may not be run in parallel!
func Connect(cfg Configuration) (*sql.DB, error) {
	migrations := cfg.Migrations
	if cfg.PostgresURL == nil {
		migrations = translatePostgresDDLToSQLite(migrations)
	} else {
		migrations = prepareDDLForPostgres(migrations)
	}
	migrations = stripWhitespace(migrations)

	//use the "go-bindata" driver for github.com/golang-migrate/migrate
	var assetNames []string
	for name := range migrations {
		assetNames = append(assetNames, name)
	}
	asset := func(name string) ([]byte, error) {
		data, ok := migrations[name]
		if ok {
			return []byte(data), nil
		}
		return nil, &os.PathError{Op: "open", Path: name, Err: errors.New("not found")}
	}

	sourceDriver, err := bindata.WithInstance(bindata.Resource(assetNames, asset))
	if err != nil {
		return nil, err
	}

	var (
		db      *sql.DB
		dbd     database.Driver
		dbdName string
	)

	if cfg.PostgresURL == nil {
		db, dbd, err = connectToSQLite(cfg.OverrideDriverName, sourceDriver)
		if err != nil {
			return nil, fmt.Errorf("cannot create SQLite in-memory DB: %s", err.Error())
		}
		dbdName = "sqlite3"
	} else {
		db, dbd, err = connectToPostgres(cfg.PostgresURL, cfg.OverrideDriverName)
		if err != nil {
			return nil, fmt.Errorf("cannot connect to Postgres: %s", err.Error())
		}
		dbdName = "postgres"
	}

	err = runMigration(migrate.NewWithInstance("go-bindata", sourceDriver, dbdName, dbd))
	if err != nil {
		return nil, fmt.Errorf("cannot apply database schema: %s", err.Error())
	}
	return db, nil
}

func connectToSQLite(driverName string, sourceDriver source.Driver) (*sql.DB, database.Driver, error) {
	if driverName == "" {
		driverName = "sqlite3-postlite"
	}
	//see FAQ in go-sqlite3 README about the connection string
	dsn := "file::memory:?mode=memory&cache=shared"
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, nil, err
	}

	//wipe leftovers from previous test runs
	//(courtesy of https://stackoverflow.com/a/548297/334761)
	for _, stmt := range []string{
		"PRAGMA writable_schema = 1;",
		"DELETE FROM sqlite_master WHERE TYPE IN ('table', 'index', 'trigger');",
		"PRAGMA writable_schema = 0;",
		"VACUUM;",
		"PRAGMA INTEGRITY_CHECK;",
	} {
		_, err := db.Exec(stmt)
		if err != nil {
			return nil, nil, err
		}
	}

	//we cannot use `db` for migrate; the sqlite3 driver for migrate gets
	//confused by the customizations in the sqlite3-postlite SQL driver
	dbForMigrate, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, nil, err
	}
	dbd, err := sqlite3.WithInstance(dbForMigrate, &sqlite3.Config{})
	return db, dbd, err
}

var dbNotExistErrRx = regexp.MustCompile(`^pq: database "([^"]+)" does not exist$`)

func connectToPostgres(url *net_url.URL, driverName string) (*sql.DB, database.Driver, error) {
	if driverName == "" {
		driverName = "postgres"
	}
	db, err := sql.Open(driverName, url.String())
	if err == nil {
		//apparently the "database does not exist" error only occurs when trying to issue the first statement
		_, err = db.Exec("SELECT 1")
	}
	if err == nil {
		//success
		dbd, err := postgres.WithInstance(db, &postgres.Config{})
		return db, dbd, err
	}
	match := dbNotExistErrRx.FindStringSubmatch(err.Error())
	if match == nil {
		//unexpected error
		return nil, nil, err
	}
	dbName := match[1]

	//connect to Postgres without the database name specified, so that we can
	//execute CREATE DATABASE
	urlWithoutDB := *url
	urlWithoutDB.Path = "/"
	db2, err := sql.Open("postgres", urlWithoutDB.String())
	if err == nil {
		_, err = db2.Exec("CREATE DATABASE " + dbName)
	}
	if err == nil {
		err = db2.Close()
	} else {
		db2.Close()
	}
	if err != nil {
		return nil, nil, err
	}

	//now the actual database is there and we can connect to it
	db, err = sql.Open("postgres", url.String())
	if err != nil {
		return nil, nil, err
	}
	dbd, err := postgres.WithInstance(db, &postgres.Config{})
	return db, dbd, err
}

func runMigration(m *migrate.Migrate, err error) error {
	if err != nil {
		return err
	}
	err = m.Up()
	if err == migrate.ErrNoChange {
		//no idea why this is an error
		return nil
	}
	return err
}

var sqlCommentRx = regexp.MustCompile(`--.*?(\n|$)`)

func stripWhitespace(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for filename, sql := range in {
		sqlWithoutComments := sqlCommentRx.ReplaceAllString(sql, "")
		out[filename] = strings.Replace(
			strings.Join(strings.Fields(sqlWithoutComments), " "),
			"; ", ";\n", -1,
		)
	}
	return out
}

var skipInPostgresRx = regexp.MustCompile(`(?ms)^\s*--\s*BEGIN\s+skip\s+in\s+postgres\s*?$.*^\s*--\s*END\s+skip\s+in\s+postgres\s*?$`)

func prepareDDLForPostgres(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for filename, sql := range in {
		//remove DDL that is only used for SQLite
		sql = skipInPostgresRx.ReplaceAllString(sql, "")
		//wrap DDL in transactions
		out[filename] = "BEGIN;\n" + strings.TrimSuffix(strings.TrimSpace(sql), ";") + ";\nCOMMIT;"
	}
	return out
}
