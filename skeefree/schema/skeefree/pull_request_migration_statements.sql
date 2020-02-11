CREATE TABLE `pull_request_migration_statements` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `pull_requests_id` bigint(20) unsigned NOT NULL,
  `migration_statement` text NOT NULL,
  `status` varchar(32) NOT NULL,
  `added_timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `pull_requests_id_idx` (`pull_requests_id`),
  KEY `status_idx` (`status`,`added_timestamp`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
