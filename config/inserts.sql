\c src
insert into t select (id+1) from t;;
insert into t2 values('{"id":"passport"}'::json);
insert into t3 values('{"id": 1}'::json);
insert into t4 values(true);
insert into t5 values(now());
insert into t5 values(null);
insert into t6 values(1.0/134.0);
insert into t6 values(1.21);
insert into t7 values('192.168.0.1'::inet, '255.255.0.0'::cidr);
insert into t8 values('1 days 22 seconds'::interval);
insert into t8 values('1 year 221 microseconds'::interval);
insert into t8 values('1 year 221 milliseconds'::interval);
insert into t9 values(0.001);
insert into t10 values('''');
insert into t11 values('{ 1, 2, 3 }');
insert into t12 values('<list><item1/></list>'::xml);
