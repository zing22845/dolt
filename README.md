## Dolt
Dolt is a relational database, i.e. it has tables, and you can execute SQL queries against those tables. It also has version control primitives that operate at the level of table cell. Thus Dolt is a database that supports fine grained value-wise version control, where all changes to data and schema are stored in commit log.

It is inspired by RDBMS and Git, and attempts to blend concepts about both in a manner that allows users to better manage, distribute, and collaborate on, data.

We also built [DoltHub](https://www.dolthub.com), a cloud based storage solution for hosting Dolt databases, that facilitates collaborative management of databases. We host public data for free!

## Installation
Obtain the appropriate archive for your operating system under [releases](https://github.com/liquidata-inc/dolt/releases):

System|Archive
---|---
64-bit Mac|dolt-darwin-amd64.tar.gz
32-bit Mac|dolt-darwin-386.tar.gz
64-bit Linux|dolt-linux-amd64.tar.gz
32-bit Linux|dolt-linux-386.tar.gz
64-bit Windows|dolt-windows-amd64.tar.gz
32-bit Windows|dolt-windows-386.tar.gz

For Unix systems extract the archive to a directory on in your path, for example:
```
$ tar -xf /your/download/location/dolt-darwin-amd64.tar.gz -C /usr/local/bin
$ ln -s /usr/local/lib/dolt /usr/local/lib/dolt/bin/dolt
```
Verify that your installation has succeeded as follows:
```
$ dolt
Valid commands for dolt are
[...]
```
Finally, setup your name and email in the global config (this should be very familiar to Git users):
```
$ dolt config --global --add user.email YOU@DOMAIN.COM
$ dolt config --global --add user.name "YOUR NAME"
```
You're all set to start getting value from Dolt!

## First Repository

Suppose we want to create a table of state populations from 1790 in Dolt, then:
```
$ mkdir state-populations
$ cd state-populations
```
Initialize the directory, and load some data:
```
$ dolt init
Successfully initialized dolt data repository.
$ dolt sql -q "create table state_populations ( state varchar, population int, primary key (state) )"
$ dolt sql -q "show tables"
+-------------------+
| tables            |
+-------------------+
| state_populations |
+-------------------+
$ dolt sql -q 'insert into state_populations (state, population) values
("Delaware", 59096),
("Maryland", 319728),
("Tennessee", 35691),
("Virginia", 691937),
("Connecticut", 237946),
("Massachusetts", 378787),
("South Carolina", 249073),
("New Hampshire", 141885),
("Vermont", 85425),
("Georgia", 82548),
("Pennsylvania", 434373),
("Kentucky", 73677),
("New York", 340120),
("New Jersey", 184139),
("North Carolina", 393751),
("Maine", 96540),
("Rhode Island", 68825)'
Rows inserted: 17
```
Now let's run some SQL against it:
```
$ dolt sql -q "select * from state_populations where state = 'New York'"
+----------+------------+
| state    | population |
+----------+------------+
| New York | 340120     |
+----------+------------+
```
Assuming you're satisfied, create a commit as follows:
```
$ dolt add .
$ dolt commit -m 'adding state populations from 1790.'
$ dolt status
On branch master
nothing to commit, working tree clean
```

You've just generated your first commit to a relational database with version control!

Alternatively, you can import CSV and PSV files, where the file type is inferred from the extension. Suppose your state populations are in a file:
```
$ head -n3 data.csv
state,population
Delaware,59096
Maryland,319728
$ dolt import table -pk=state state_populations data.csv
```
Note if you do not have a file extension, i.e. your file is called `data`, Dolt will think you are trying to import from another table and thus not behave in the way you expect.

## Modifying Data

Override the table with the contents of another file:
```
$ dolt table import --update-table <table> <csv_file>
```
To add or update rows, use the SQL interface:
```
$ dolt sql --query 'INSERT INTO state_populations VALUES ("My State", 0)'
Rows inserted: 1
$ dolt sql --query 'UPDATE state_populations SET population=1 where state="My State"'
Rows updated: 1
```
Now you can see that you have changes to a table that are not staged for commit, in the same way an edited file is not automatically staged for commit:
```
$ dolt status
On branch master
Changes not staged for commit:
  (use "dolt add <table>" to update what will be committed)
  (use "dolt checkout <table>" to discard changes in working directory)
	modified:       state_populations
$ dolt diff
diff --dolt a/state_populations b/state_populations
--- a/state_populations @ u0b70pnkhsl1s6rmc6o44nlphdslgipj
+++ b/state_populations @ gbk38aq4o35hfj692fnb32apbpfpamu0
+-----+----------+------------+
|     | state    | population |
+-----+----------+------------+
|  +  | My State | 1          |
+-----+----------+------------+
```
If you're happy with the changes, you can go ahead and commit them:
```
$ dolt add state_populations
$ dolt commit -m "Added 'My State'"
```
Well done, you updated a table, and committed your changes!

## Simple Branching Workflow

When making changes it is advisable to create a new branch which will serve as the workspace for your changes. Choose a short branch name that describes the work you are planning to do. Again, this works identically to git:

    dolt checkout -b <branch>

Once you've made your changes, add and commit the modified tables like you did previously.  Once your work is done and you are ready to get all your changes back into the master branch run:

    dolt checkout master
    dolt merge <branch>

Then you'll need to add and commit the merged data:

    dolt add .
    dolt commit -m "Merged work from <branch> into master"


## Adding Remotes

Dolt supports remotes in a similar manner to Git. Liquidata, the company behind Dolt, also created DoltHub, a hosting service for Dolt databases. In the following we use DoltHub as an example for setting up a remote.

If you haven't done so already, setting up your default servers will make it easier to add and clone remotes
```
$ dolt login
[...]
```
Which should open a browser window where you can create a credential for HTTPS. Upon successful creation the following will appear in the shell:
```
Key successfully associated with user: youusername email you@youremail.com

```
Next you'll want to make sure you've created the remote at https://www.dolthub.com .  Once created you can add the remote.  As an example, if the repository is created under an organization named "org", with the name "repo" you could add the remote like so:

    dolt remote add origin org/repo

Once the remote is added viewing the remote using the command:

    dolt remote -v

should show something like this:

    origin https://doltremoteapi.dolthub.com/org/repo

If you've created the repository on the web and added a remote, you should be able to push the branch "master" to the remote named "origin" like so

    dolt push origin master

Once that is succeeded others can clone the repository (assuming you've given them permission.)

    dolt clone org/repo


## Other remotes

dolt also supports directory, aws, and gcs based remotes:

  * file - you can use a directory as a remote that can be pushed to, cloned, and purlled from just like any other remote by providing a file uri for the directory

    dolt remote add <remote> file:///Users/xyz/abs/path/

  * aws - you can use your aws cloud resources directly (If you are interested in trying this contact the dolt team directly as we are still writing documentation).

    dolt remote add <remote> aws://dynamo-table:s3-bucket/database

  * gs - you can use a GCS bucket as well.  Setup here should be straight forward.  You'll need to create a gcs bucket, and you'll need to use the gcloud auth login command in order to setup your credentials.

    dolt remote add <remote> gs://gcs-bucket/database

## Issues
If you have any issues with Dolt, find any bugs, or simply have a question, feel free to file an issue!

# Credits and License
The implementation of Dolt makes use of code and ideas from
[noms](https://github.com/attic-labs/noms).

Dolt is licensed under the Apache License, Version 2.0. See
[LICENSE](https://github.com/liquidata-inc/dolt/blob/master/LICENSE) for
details.
