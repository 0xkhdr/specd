| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| ⬜ T1 | builder | demo.txt | - | true | first task passes |
| ⬜ T2 | builder | demo2.txt | T1 | true | second task waits |
