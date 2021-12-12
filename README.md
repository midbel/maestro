# maestro

maestro helps to organize all the tasks and/or commands that need to be performed regularly in a project whatever its nature. It could be the development of a program, administration of a single server or a set of virtual machines,...

### maestro file

#### comment

#### meta

* AUTHOR:  author of the maestro file
* EMAIL:   e-mail of the author of the maestro file
* VERSION; current version of the maestro file
* USAGE;   short help message of the maestro file
* HELP;    longer description of the maestro file and description of its commands/usage

* DUPLICATE; behaviour of maestro when it encounters a command with a name already registered. The possible values are:
   - error: throw an error if a command with the same name is already registered
   - replace: replace the previous definition of a command by the new one
   - append:  make the two commands as one

* TRACE:
* WORKDIR:

* ALL:
* DEFAULT:
* BEFORE:
* AFTER:
* ERROR:
* SUCCESS:

* SSH_USER:
* SSH_PASSWORD:
* SSH_PUBKEY:
* SSH_KNOWN_HOSTS:

#### variables

#### instructions

##### include
##### export
##### alias
##### delete

#### Command

general syntax:

```
[%]command([property,...]): [dependency...] {
  [modifier...]script
}
```

##### command properties

* short:
* help:
* tag:
* alias:

* workdir:
* retry:
* timeout:
* error:

* user:
* group:

* options:
* args:

* hosts:

##### command options and arguments

##### command dependencies

##### command help

##### command script

#### example

```
.VERSION = "0.1.0"
.DEFAULT = test
.ALL     = test
.AUTHOR  = midbel
.EMAIL   = "noreply@midbel.dev"
.HELP    = <<HELP
simple is a sample maestro file that can be used for any Go project.
It provides commands to automatize building, checking and testing
your Go project.
HELP

package = "github.com/midbel/maestro"

test(
	short = "run test in current directory",
	tag   = build test,
): {
  # run go test
	go clean -testcache
	@go test -cover $package
	@go test -cover "$package/shlex"
	@go test -cover "$package/shell"
}
```

### maestro shell
