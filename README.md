# maestro

### maestro file

#### comment

#### meta

* AUTHOR;
* EMAIL;
* VERSION;
* USAGE;
* HELP;

* DUPLICATE;

* ECHO:
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
