# maestro

maestro helps to organize all the tasks and/or commands that need to be performed regularly in a project whatever its nature. It could be the development of a program, administration of a single server or a set of virtual machines,...

### maestro file

#### comment

a hash symbol marks the rest of the line as a comment (except when inside of a string).

```
# a comment
ident = value # another comment
```

inside command scripts, comments are written in the same way as elsewhere in the document.

#### basic types

maestro supports only three different primitive types:

* `literal`: a literal is a sequence of characters that start with a letter and then contains only letters (lower/uppercase), digits and/or underscore.
* `string`: a string is a sequence of characters that starts with a single or a double quote and ends with the same opening quote. There are no differences in the way characters inside the string are process regarding of the delimiting quotes.
* `boolean`: they are just the commons values we are used - true or false (always lowercase).

However, in some circumstances, maestro expects that values written as literal and/or string can be casted to integer values and/or duration values.

There is also a special case for boolean values. Indeed, depending of the context a boolean value will be considered as boolean but in some other case, the value of the boolean will be treated as a literal value.

Finally, maestro supports also multiline strings by using a syntax identical to the one of heredoc string of `bash`

some examples:
```
boolean = true
ident   = literal
single_quote = 'the quick brown fox jumps over the lazy dog'  
double_quote = "the quick brown fox jumps over the lazy dog"
heredoc = <<HELP
the quick brown fox
jumps over
the lazy dog!
HELP
```

maestro supports also a list of strings type (kind of array of string). To declare it, just provide a sequence of values separated by blank character (space or tab)

example:
```
list = first second third
```

#### variables

```
mode = dev # single value variable
package = shell shlex wrap # multi values variable
bindir = $bin # set value of variable bin to bindir

expansion = $(echo foo bar)
```

#### meta

* `.AUTHOR`: author of the maestro file
* `.EMAIL`: e-mail of the author of the maestro file
* `.VERSION`: current version of the maestro file
* `.USAGE`: short help message of the maestro file
* `.HELP`: longer description of the maestro file and description of its commands/usage
* `.DUPLICATE`: behaviour of maestro when it encounters a command with a name already registered. The possible values are:
  - error: throw an error if a command with the same name is already registered
  - replace: replace the previous definition of a command by the new one
  - append:  make the two commands as one
* `.TRACE`: enable/disabled tracing information
* `.WORKDIR`: set the working directory of maestro to the given path
* `.ALL`: list of commands that will be executed when calling `maestro all`
* `.DEFAULT`: name of the command that will be executed when calling `maestro` without argument or by calling `maestro default`
* `.BEFORE`: list of commands that will always be executed before the called command and its dependencies
* `.AFTER`: list of commands that will always be executed after the called command has finished whatever its exit status
* `.ERROR`: list of commands that will be executed after the called command has finished and its exit status is non zero (failure)
* SUCCESS: list of commands that will be executed after the called command has finished and its exit status is zero (success)
* `.SSH_USER`: username to use when executing command to remote server(s) via SSH
* `.SSH_PASSWORD`: password to use when executing command to remote server(s) via SSH
* `.SSH_PARALLEL`: number of instance of a command that will be executed simultaneously
* `.SSH_PUBKEY`: public key file to use when executing command to remote server(s) via SSH
* `.SSH_KNOWN_HOSTS`: known_hosts file to use to validate remote server(s) key

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

* `short`: short description of a command
* `help`: longer description of a command.
* `tag`:  list of tags to help categorize a command in comparison with other
* `alias`: list of alternative name of a command
* `workdir`: set working directory for the command
* `retry`: number of attempts to run a command
* `timeout`: maximum time given to a command in order to fully complete
* `error`: behavior of maestro when the command encounters an error. The possible values are:
  - silent: ignore all error
  - error: return the first error encounters
* `user`: list of users allowed to run a command
* `group`: list of groups allowed to run a command
* `options`: list of list that describes the options accepted by a command
* `args`: list of names that describes the arguments required by a command
* `hosts`: list of remote servers where a command can be executed. The expected syntax is host:port

##### command options and arguments

maestro allows to define the options and/or arguments that a command can accept. In the properties section of a command, there is only needs to specify the `options` and/or the `args` properties.

The `options` property accept a list of list with the following property:

* `short`: short option
* `long`: long option
* `desc`: description of the option
* `flag`: wheter the option is a flag or is expecting a value
* `required`: wheter a value should be provided
* `default`: default value to use if the option is not set

For the `args` property, only a list of name is needed. The command when executed will expect that the number of arguments given matched the number of arguments given in the list. If the `args` property is not defined then any given arguments will be given to the command without checking its number.

example
```
action(
	short   = "sample action", # inline comment
	tag     = example,
	options = (
		short    = "a",
		long     = "all",
		flag     = true,
	), (
		short    = "b",
		long     = "bind",
    required = true,
	)
): {
	script...
}
```

##### command dependencies

##### command help

##### command script

###### modifiers

a script modifier is a way to attach specific behaviour to the script when it is executed or after it has finished:

* `-`: ignore errors if script ends with a non-zero exit code
* `!`: reverse the exit code of a command. If the exit code is zero then a non zero value is returned
* `@`: print on stdout the command being executed
* `<`: copy the full script of a command defined elsewhere in the maestro file

multiple operators can be used simultaneously. except for the copy modifier that can only be used alone

examples:

```
!-echo foobar
@echo foobar
<copy
```

###### repeat macro

```
.repeat(foo bar) {
  echo <iter> <var>
}
```
will be transform by maestro into
```
echo 1 foo
echo 2 bar
```

###### sequence macro

the sequence macro can be used to transformed a multiline sequence of a command as a single command made of a list of command.

eg:
```
.sequence {
  echo foo
  echo bar
}
```
will be transformed into:
```
echo foo; echo bar;
```

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

### local command execution

### remote command execution
