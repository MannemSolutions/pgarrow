create database src;
\c src
DROP PUBLICATION IF EXISTS pgarrow;
CREATE PUBLICATION pgarrow FOR ALL TABLES;
select pg_create_logical_replication_slot('pgarrow', 'pgoutput');
create table t (id int primary key, name text);
create table t2 (j json);
create table t3 (j jsonb);
create table t4 (b bool);
create table t5 (t timestamptz);
create table t6 (f float);
create table t7 (i inet, c cidr);
create table t8 (i interval);
create table t9 (n numeric);
create table t10 (b bytea);
create table t11 (l line);
create table t12 (x xml);

create database dest;
\c dest
create table t (id int primary key, name text);
create table t2 (j json);
create table t3 (j jsonb);
create table t4 (b bool);
create table t5 (t timestamptz);
create table t6 (f float);
create table t7 (i inet, c cidr);
create table t8 (i interval);
create table t9 (n numeric);
create table t10 (b bytea);
create table t11 (l line);
create table t12 (x xml);
