# Tasks - demo

| id | role | files | depends-on | verify | acceptance |
| --- | --- | --- | --- | --- | --- |
| T1 | craftsman | `demo/one.txt` | - | `printf one` | first root task |
| T2 | craftsman | `demo/two.txt` | - | `printf two` | second root task |
| T3 | craftsman | `demo/three.txt` | T1 | `printf three` | waits for T1 |
| T4 | craftsman | `demo/four.txt` | T2 | `printf four` | waits for T2 |
