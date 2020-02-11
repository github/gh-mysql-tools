CREATE TABLE `repo_production_mapping` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `org` varchar(128) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `hint` varchar(128) NOT NULL,
  `mysql_cluster` varchar(128) NOT NULL,
  `mysql_schema` varchar(128) NOT NULL,
  `added_timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `org_repo_uidx` (`org`,`repo`,`hint`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4;
