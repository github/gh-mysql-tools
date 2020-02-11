CREATE TABLE `repositories` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `org` varchar(128) NOT NULL,
  `repo` varchar(128) NOT NULL,
  `owner` varchar(128) NOT NULL,
  `autorun` tinyint(1) NOT NULL DEFAULT 0,
  `added_timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `org_repo_uidx` (`org`,`repo`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4;
