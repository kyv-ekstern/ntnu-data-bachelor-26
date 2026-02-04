create table if not exists anomaly_groups (
  id bigint generated always as identity primary key,
  type text not null,
  mmsi bigint not null,
  started_at timestamp not null,
  last_activity_at timestamp not null,
  position geometry(Point, 4326) not null
);

create table if not exists anomalies (
  id bigint generated always as identity,
  type text not null,
  metadata jsonb,
  created_at timestamp not null,
  mmsi bigint,
  anomaly_group_id bigint,
  data_source varchar(255) default 'UNKNOWN' :: character varying not null,
  foreign key (anomaly_group_id) references anomaly_groups(id)
);