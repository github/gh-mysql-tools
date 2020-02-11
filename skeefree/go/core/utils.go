package core

type PRMigrationsMap map[int64]([]Migration)

func MapPRMigrations(migrations []Migration) (prMigrationsMap PRMigrationsMap, orderedPRIds []int64) {
	prMigrationsMap = make(PRMigrationsMap)
	for _, m := range migrations {
		if _, ok := prMigrationsMap[m.PR.Id]; !ok {
			prMigrationsMap[m.PR.Id] = []Migration{}
			orderedPRIds = append(orderedPRIds, m.PR.Id)
		}
		prMigrationsMap[m.PR.Id] = append(prMigrationsMap[m.PR.Id], m)
	}
	return prMigrationsMap, orderedPRIds
}

func EvaluateStrategy(prStatement PullRequestMigrationStatement, allowAuto bool) MigrationStrategy {
	if allowAuto {
		switch prStatement.GetMigrationType() {
		case CreateTableMigrationType, DropTableMigrationType:
			return MigrationStrategyDirect
		case AlterTableMigrationType:
			return MigrationStrategyGhost
		}
	}
	return MigrationStrategyManual
}
