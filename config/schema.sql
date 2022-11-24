create database src;
\c src
DROP PUBLICATION IF EXISTS pgarrow;
CREATE PUBLICATION pgarrow FOR ALL TABLES;
create table t (id int primary key, name text);
insert into t values(coalesce((select max(id) from t),0)+1, 'foo');
create database dest;
\c dest
create table t (id int primary key, name text);
