# maestro

maestro helps to organize all the tasks and/or commands that need to be performed regularly in a project whatever its nature. It could be the development of a program, administration of a single server or a set of virtual machines,...

### maestro file

#### comment

#### meta

* .AUTHOR: author of the maestro file
* .EMAIL: e-mail of the author of the maestro file
* .VERSION; current version of the maestro file
* .USAGE; short help message of the maestro file
* .HELP; longer description of the maestro file and description of its commands/usage
* .DUPLICATE; behaviour of maestro when it encounters a command with a name already registered. The possible values are:
   - error: throw an error if a command with the same name is already registered
   - replace: replace the previous definition of a command by the new one
   - append:  make the two commands as one
* .TRACE: enable/disabled tracing information
* .WORKDIR: set the working directory of maestro to the given path
* .ALL: list of commands that will be executed when calling `maestro all`
* .DEFAULT: name of the command that will be executed when calling `maestro` without argument or by calling `maestro default`
* .BEFORE: list of commands that will always be executed before the called command and its dependencies
* .AFTER: list of commands that will always be executed after the called command has finished whatever its exit status
* .ERROR: list of commands that will be executed after the called command has finished and its exit status is non zero (failure)
* SUCCESS: list of commands that will be executed after the called command has finished and its exit status is zero (success)
* .SSH_USER: username to use when executing command to remote server(s) via SSH
* .SSH_PASSWORD: password to use when executing command to remote server(s) via SSH
* .SSH_PARALLEL: number of instance of a command that will be executed simultaneously
* .SSH_PUBKEY: public key file to use when executing command to remote server(s) via SSH
* .SSH_KNOWN_HOSTS: known_hosts file to use to validate remote server(s) key

#### variables

#### instructions

##### include
##### export
##### alias
##### delete

#### Command

Commands are at the heart of maestro. They are composed of four parts:

* the command name that serve as uniquely identify the command
* properties are used by maestro to generate help of a command but they are also used by maestro to control the behaviour of the commands in case of errors, long running tasks,...
* dependendies are a list of command's name that should be executed every time before a specific command is executed
* the command script that are the actual code that will be run by maestro

general syntax:

```
[%]command([property,...]): [dependency...] {
  [modifier...]script
}
```

##### command properties

* short: short description of a command
* help: longer description of a command.
* tag:  list of tags to help categorize a command in comparison with other
* alias: list of alternative name of a command
* workdir: set working directory for the command
* retry: number of attempts to run a command
* timeout: maximum time given to a command in order to fully complete
* error: behavior of maestro when the command encounters an error
* user: list of users allowed to run a command
* group: list of groups allowed to run a command
* options: list of objects that describes the options accepted by a command
* args: list of names that describes the arguments required by a command
* hosts: list of remote servers where a command can be executed

##### command options and arguments

##### command dependencies

##### command help

##### command script

###### modifiers

###### macros

#### example

```makefile
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
