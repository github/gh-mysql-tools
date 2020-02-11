CREATE TABLE `service_election` (
  `anchor` int(10) unsigned NOT NULL,
  `service_id` varchar(128) NOT NULL,
  `last_seen_active` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`anchor`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
