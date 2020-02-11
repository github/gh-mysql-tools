package core

import (
	"fmt"
	"strings"
	"testing"

	test "github.com/openark/golib/tests"
)

func TestParseSkeemaDiffStatements(t *testing.T) {
	{
		b := ``
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 0)
		test.S(t).ExpectEquals(info.FileName, "")
		test.S(t).ExpectEquals(info.SchemaName, "")
	}
	{
		b := `
-- skeema:ddl:begin
ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;
-- skeema:ddl:end

		`
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 0)
		test.S(t).ExpectEquals(info.FileName, "")
		test.S(t).ExpectEquals(info.SchemaName, "")
	}
	{
		b := `
<!-- skeema:diff -->
-- skeema:ddl:begin
ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;
-- skeema:ddl:end

		`
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 1)
		test.S(t).ExpectEquals(info.FileName, "")
		test.S(t).ExpectEquals(info.SchemaName, "")
		test.S(t).ExpectEquals(info.Statements[0], `ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;`)
	}
	{
		b := `
<!-- skeema:magic:comment -->
-- skeema:ddl:begin
ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;
-- skeema:ddl:end

		`
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 1)
		test.S(t).ExpectEquals(info.FileName, "")
		test.S(t).ExpectEquals(info.SchemaName, "")
		test.S(t).ExpectEquals(info.Statements[0], `ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;`)
	}
	{
		b := `
<!-- skeema:magic:comment -->
-- skeema:ddl:use some_schema
-- skeema:ddl:begin
ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;
-- skeema:ddl:end
		`
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 1)
		test.S(t).ExpectEquals(info.FileName, "")
		test.S(t).ExpectEquals(info.SchemaName, "some_schema")
		test.S(t).ExpectEquals(info.Statements[0], `ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;`)
	}
	{
		b := strings.ReplaceAll(`
<!-- skeema:magic:comment -->
-- skeema:ddl:use §some_schema§
-- skeema:ddl:begin
ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;
-- skeema:ddl:end
		`, "§", "`")
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 1)
		test.S(t).ExpectEquals(info.FileName, "")
		test.S(t).ExpectEquals(info.SchemaName, "some_schema")
		test.S(t).ExpectEquals(info.Statements[0], `ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;`)
	}
	{
		b := strings.ReplaceAll(`
<!-- skeema:magic:comment -->
-- skeema:ddl:use §skeema-ci:some_schema§
-- skeema:ddl:begin
ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;
-- skeema:ddl:end
		`, "§", "`")
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 1)
		test.S(t).ExpectEquals(info.FileName, "")
		test.S(t).ExpectEquals(info.SchemaName, "some_schema")
		test.S(t).ExpectEquals(info.Statements[0], `ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;`)
	}
	{
		b := strings.ReplaceAll(`
<!-- skeema:magic:comment -->
-- skeema:ddl:use §skeema-ci:some_schema§;
-- skeema:ddl:begin
ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;
-- skeema:ddl:end
		`, "§", "`")
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 1)
		test.S(t).ExpectEquals(info.FileName, "")
		test.S(t).ExpectEquals(info.SchemaName, "some_schema")
		test.S(t).ExpectEquals(info.Statements[0], `ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;`)
	}
	{
		b := `
<!-- skeema:diff -->
-- skeema:diff:file collab-structure.sql
-- skeema:ddl:begin
ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;
-- skeema:ddl:end

		`
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 1)
		test.S(t).ExpectEquals(info.FileName, "collab-structure.sql")
		test.S(t).ExpectEquals(info.SchemaName, "")
		test.S(t).ExpectEquals(info.Statements[0], `ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;`)
	}

	{
		b := `
<!-- skeema:magic:comment -->
-- skeema:ddl:begin
ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;
-- skeema:ddl:end
-- skeema:ddl:begin
CREATE TABLE table_0 (
id int(10) unsigned NOT NULL AUTO_INCREMENT,
name varchar(128) NOT NULL,
PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- skeema:ddl:end
		`
		info := ParseSkeemaDiffStatements(b)
		test.S(t).ExpectEquals(len(info.Statements), 2)
		test.S(t).ExpectEquals(info.FileName, "")
		test.S(t).ExpectEquals(info.SchemaName, "")
		test.S(t).ExpectEquals(info.Statements[0], `ALTER TABLE sample_data ADD COLUMN i int(11) NOT NULL DEFAULT '7' AFTER time_updated;`)
		test.S(t).ExpectEquals(info.Statements[1], `CREATE TABLE table_0 (
id int(10) unsigned NOT NULL AUTO_INCREMENT,
name varchar(128) NOT NULL,
PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`)
	}
}

func TestdissectDropTableStatement(t *testing.T) {
	{
		statement := ""
		_, err := dissectDropTableStatement(statement)
		test.S(t).ExpectNotNil(err)
	}
	{
		statement := "DROP TABLE zzz"
		_, err := dissectDropTableStatement(statement)
		test.S(t).ExpectNotNil(err)
	}
	{
		statement := "DROP TABLE `zzz`"
		tableName, err := dissectDropTableStatement(statement)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(tableName, "zzz")
	}
	{
		statement := "DROP TABLE `zzz` ;"
		tableName, err := dissectDropTableStatement(statement)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectEquals(tableName, "zzz")
	}
}

func TestdissectAlterTableStatement(t *testing.T) {
	{
		statement := ""
		_, _, _, err := dissectAlterTableStatement(statement)
		test.S(t).ExpectNotNil(err)
	}
	{
		statement := "ALTER TABLE zzz"
		_, _, _, err := dissectAlterTableStatement(statement)
		test.S(t).ExpectNotNil(err)
	}
	{
		statement := "ALTER TABLE `zzz`"
		_, _, _, err := dissectAlterTableStatement(statement)
		test.S(t).ExpectNotNil(err)
	}
	{
		statement := "ALTER TABLE `zzz` ADD COLUMN `i` INT NOT NULL DEFAULT 0"
		tableName, alter, hasDropColumn, err := dissectAlterTableStatement(statement)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectFalse(hasDropColumn)
		test.S(t).ExpectEquals(tableName, "zzz")
		test.S(t).ExpectEquals(alter, "ADD COLUMN `i` INT NOT NULL DEFAULT 0")
	}
	{
		statement := "ALTER TABLE `zzz` ADD COLUMN `i` INT NOT NULL DEFAULT 0, ADD INDEX i_idx(i)"
		tableName, alter, hasDropColumn, err := dissectAlterTableStatement(statement)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectFalse(hasDropColumn)
		test.S(t).ExpectEquals(tableName, "zzz")
		test.S(t).ExpectEquals(alter, "ADD COLUMN `i` INT NOT NULL DEFAULT 0, ADD INDEX i_idx(i)")
	}
	{
		statement := "ALTER TABLE `zzz` ADD COLUMN `i` INT NOT NULL DEFAULT 0, DROP COLUMN `j`"
		tableName, alter, hasDropColumn, err := dissectAlterTableStatement(statement)
		test.S(t).ExpectNil(err)
		test.S(t).ExpectTrue(hasDropColumn)
		test.S(t).ExpectEquals(tableName, "zzz")
		test.S(t).ExpectEquals(alter, "ADD COLUMN `i` INT NOT NULL DEFAULT 0, DROP COLUMN `j`")
	}
}

func TestGetTableNames(t *testing.T) {
	{
		tbl := "some_table"
		test.S(t).ExpectEquals(GetSafeTableNameWithSuffix(tbl, "DRP"), "some_table_DRP")
		test.S(t).ExpectEquals(GetSafeTableNameWithSuffix(fmt.Sprintf("_%s", tbl), "DRP"), "_some_table_DRP")
	}
	{
		tbl := "a123456789012345678901234567890123456789012345678901234567890"
		test.S(t).ExpectEquals(GetSafeTableNameWithSuffix(tbl, "DRP"), "a12345678901234567890123456789012345678901234567890123456789_DRP")
		test.S(t).ExpectEquals(GetSafeTableNameWithSuffix(fmt.Sprintf("_%s", tbl), "DRP"), "_a1234567890123456789012345678901234567890123456789012345678_DRP")
	}
}
