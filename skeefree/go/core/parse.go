package core

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	diffStatementRegexp = regexp.MustCompile("(?s)\n-- skeema:ddl:begin(.*?)\n-- skeema:ddl:end")
	diffFileRegexp      = regexp.MustCompile("(?s)\n-- skeema:diff:file (.*?)\n")
	diffUseRegexp       = regexp.MustCompile("(?s)\n-- skeema:ddl:use (.*?)\n")
	alterDatabaseRegexp = regexp.MustCompile("ALTER DATABASE `(.*?)` (.+)$")
	createTableRegexp   = regexp.MustCompile("CREATE TABLE `(.*?)`")
	dropTableRegexp     = regexp.MustCompile("DROP TABLE `(.*?)`")
	alterTableRegexp    = regexp.MustCompile("ALTER TABLE `(.*?)` (.+)$")
)

const MaxTableNameLength = 64

type SkeemaDiffInfo struct {
	Statements []string
	FileName   string
	SchemaName string
}

// ParseSkeemaDiffStatements parses the magic text injected into a PR magic comment, e.g.
/*
<!-- skeema:magic:comment -->
-- skeema:diff
-- skeema:ddl:use skeema:my_schema;
-- skeema:ddl:begin
ALTER TABLE my_table ADD COLUMN dummy_shlomi_noach tinyint(4) NOT NULL DEFAULT '0';
-- skeema:ddl:end
-- skeema:ddl:begin
CREATE TABLE a_dummy_table_do_not_merge_me (
id int(11) NOT NULL AUTO_INCREMENT,
just_an_example int(11) DEFAULT NULL,
ignore_me int(11) NOT NULL,
im_not_here int(11) DEFAULT NULL,
someone_else_did_it varchar(40) DEFAULT NULL,
i_didnt_break_production_today int(11) DEFAULT NULL,
PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
-- skeema:ddl:end
*/
func ParseSkeemaDiffStatements(commentBody string) (info *SkeemaDiffInfo) {
	info = &SkeemaDiffInfo{Statements: []string{}}

	tokens := strings.Split(commentBody, "<!-- skeema:magic:comment -->")
	if len(tokens) < 2 {
		tokens = strings.Split(commentBody, "<!-- skeema:diff -->")
		if len(tokens) < 2 {
			return info
		}
	}
	skeemaDiffClause := tokens[len(tokens)-1]

	if submatch := diffFileRegexp.FindStringSubmatch(skeemaDiffClause); len(submatch) > 0 {
		info.FileName = strings.TrimSpace(submatch[1])
	}
	if submatch := diffUseRegexp.FindStringSubmatch(skeemaDiffClause); len(submatch) > 0 {
		schema := submatch[1]
		schema = strings.TrimSpace(schema)
		schema = strings.Trim(schema, ";")
		schema = strings.TrimSpace(schema)
		schema = strings.Trim(schema, "`")
		if tokens := strings.Split(schema, ":"); len(tokens) > 1 {
			schema = tokens[len(tokens)-1]
		}
		info.SchemaName = schema
	}
	allSubmatch := diffStatementRegexp.FindAllStringSubmatch(skeemaDiffClause, -1)
	for _, submatch := range allSubmatch {
		info.Statements = append(info.Statements, strings.TrimSpace(submatch[1]))
	}

	return info
}

func dissectCreateTableStatement(statement string) (tableName string, err error) {
	submatch := createTableRegexp.FindStringSubmatch(statement)
	if len(submatch) == 0 {
		return "", fmt.Errorf("cannot dissect CREATE statement: %s", statement)
	}
	return submatch[1], nil
}

func dissectDropTableStatement(statement string) (tableName string, err error) {
	submatch := dropTableRegexp.FindStringSubmatch(statement)
	if len(submatch) == 0 {
		return "", fmt.Errorf("cannot dissect DROP statement: %s", statement)
	}
	return submatch[1], nil
}

func dissectAlterTableStatement(statement string) (tableName string, alter string, hasDropColumn bool, err error) {
	submatch := alterTableRegexp.FindStringSubmatch(statement)
	if len(submatch) == 0 {
		return "", "", false, fmt.Errorf("cannot dissect ALTER statement: %s", statement)
	}
	tableName = submatch[1]
	alter = submatch[2]
	hasDropColumn = strings.Contains(alter, "DROP COLUMN")
	return tableName, alter, hasDropColumn, nil
}

func dissectAlterDatabaseStatement(statement string) (databaseName string, alter string, err error) {
	submatch := alterDatabaseRegexp.FindStringSubmatch(statement)
	if len(submatch) == 0 {
		return databaseName, alter, fmt.Errorf("cannot dissect ALTER DATABASE statement: %s", statement)
	}
	databaseName = submatch[1]
	alter = submatch[2]
	return databaseName, alter, nil
}

func GetSafeTableNameWithSuffix(baseName string, suffix string) string {
	name := fmt.Sprintf("%s_%s", baseName, suffix)
	if len(name) <= MaxTableNameLength {
		return name
	}
	extraCharacters := len(name) - MaxTableNameLength
	return fmt.Sprintf("%s_%s", baseName[0:len(baseName)-extraCharacters], suffix)
}
